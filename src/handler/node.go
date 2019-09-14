package handler

import (
	"bufio"
	"bytes"
	"conf"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"logger"
	"net/http"
	"os"
	"path"
	"strconv"
	"time"
)

// This internal type is used for controlling internal node's goroutines behaviour.
type goroutineControlEvent struct {
	MustBeStopped bool
}

// Represents GEO engine node in the handler.
// Handles reading and writing of the fifo files.
type Node struct {
	eventsGoroutineControlChannel chan *goroutineControlEvent
}

func NewNode() *Node {
	return &Node{
		eventsGoroutineControlChannel: nil,
	}
}

// Attempts to start internal node process.
//
// Returns error in case if internal node process (geo engine client) failed to start properly,
// or connection to events fifo files can't be established correctly.
func (node *Node) AttachEventsMonitor() error {
	// Node instance must wait some time for the child process to start listening for commands.
	// this timeout specifies how long it would wait.

	CHILD_PROCESS_SPAWN_TIMEOUT_SECONDS := 1

	eventsControlEventsChanel := make(chan *goroutineControlEvent, 1)
	eventsGoroutinesErrorsChanel := make(chan error, 1)
	go node.beginReceiveEvents(
		conf.Params.Handler.NodeDirPath,
		CHILD_PROCESS_SPAWN_TIMEOUT_SECONDS,
		eventsControlEventsChanel,
		eventsGoroutinesErrorsChanel)

	// Now node handler must wait for the success response from the internal goroutines.
	// In case of no response, or response wasn't received in specified timeout -
	// error would be reported, and internal goroutines would be finished.
	CHILD_PROCESS_MAX_SPAWN_TIMEOUT_SECONDS := CHILD_PROCESS_SPAWN_TIMEOUT_SECONDS * 10
	select {
	case eventsError := <-eventsGoroutinesErrorsChanel:
		{
			return wrap("Can't start events receiving from the child process", eventsError)
		}

	case <-time.After(time.Second * time.Duration(CHILD_PROCESS_MAX_SPAWN_TIMEOUT_SECONDS)):
		{
			// There are no errors from the internal goroutines was reported for specified period of time.
			// It is assumed, that all goroutines were started well and the child process is executed normally.
			break
		}
	}

	// It seems, that child process started well.
	// Now the process descriptor must be transferred to the top, for further control.
	node.eventsGoroutineControlChannel = eventsControlEventsChanel
	node.logInfo("Attached")
	return nil
}

func openFifoFileForReading(fifoPath string, node *Node) (*os.File, error) {
	var fifo *os.File
	var counter int8 = 1
	var err error
	logger.Info("Try open " + fifoPath)
	for {
		fifo, err = os.OpenFile(fifoPath, os.O_RDONLY, 0600)
		if err != nil {
			counter++
			if counter == 5 {
				node.logError("Max tries count expired. Report error and exit")
				return fifo, err
			}
			node.logError("Can't open " + fifoPath + " for reading. Details: " + err.Error())
			node.logError("Wait 3s before repeat")
			time.Sleep(time.Second * 3)
			continue
		}
		break
	}
	logger.Info("fifo file opened")
	return fifo, err
}

func (node *Node) beginReceiveEvents(
	nodeWorkingDirPath string,
	initialStartupDelaySeconds int,
	controlEvents chan *goroutineControlEvent,
	errorsChannel chan error) {

	// Give process some time to open events.fifo for writing.
	time.Sleep(time.Second * time.Duration(initialStartupDelaySeconds))

	eventsFIFOPath := path.Join(nodeWorkingDirPath, "fifo", conf.Params.Service.EventsFile)
	fifo, err := openFifoFileForReading(eventsFIFOPath, node)
	if err != nil {
		node.logError("Can't open " + eventsFIFOPath + " for reading. Details: " + err.Error())
		errorsChannel <- wrap("Can't open "+eventsFIFOPath+" file for reading", err)
		return
	}

	reader := bufio.NewReader(fifo)
	for {
		// In case if this goroutine receives shutdown event -
		// process it and stop reading results.
		if len(controlEvents) > 0 {
			event := <-controlEvents
			if event.MustBeStopped {
				node.logError("Events receiving goroutine was finished by the external signal.")
				fifo.Close()
				return
			}
		}

		// Results are divided by "\n", so it is possible to read entire line from file.
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				time.Sleep(time.Millisecond * 5)
				continue
			}

			node.logError("Error occurred on event reading. Details are: " + err.Error())
			continue
		}

		// In some cases, redundant '\n' is returned as a result.
		// This erroneous results must be ignored.
		if len(line) == 1 && line[0] == '\n' {
			continue
		}
		node.logDebug("Received event: " + string(line))

		event := EventFromRawInput(line)
		if event.Error != nil {
			node.logError("Invalid event occurred. Details are: \"" + string(line) + "\". Dropped")
			continue
		}

		node.notifyServicesAboutEvent(event)
	}
}

type Topology struct {
	Node       string   `json:"hash"`
	Neighbors  []string `json:"neighbors"`
	Equivalent uint32   `json:"equivalent"`
}

type TrustLine struct {
	Source      string `json:"source"`
	Destination string `json:"destination"`
	Equivalent  uint32 `json:"equivalent"`
}

type Payment struct {
	Source          string     `json:"coordinator"`
	Destination     string     `json:"receiver"`
	Equivalent      uint32     `json:"equivalent"`
	TransactionUUID string     `json:"transaction_uuid"`
	Paths           [][]string `json:"paths"`
}

func (node *Node) notifyServicesAboutEvent(event *Event) {
	switch event.Code {
	case 0:
		logger.Info("Topology event")
		if len(event.Tokens) < 3 {
			node.logError("Invalid tokens count on topology event")
			return
		}
		neighborsCount, err := strconv.Atoi(event.Tokens[2])
		if err != nil {
			node.logError("Invalid neighbors count token on topology event. Details: " + err.Error())
			return
		}
		equivalent, err := strconv.Atoi(event.Tokens[0])
		if err != nil {
			node.logError("Invalid equivalent parameter on topology event. Details: " + event.Tokens[0])
			return
		}
		topology := Topology{
			Node:       convertToSHA256Hash(event.Tokens[1]),
			Equivalent: uint32(equivalent)}
		for idx := 0; idx < neighborsCount; idx++ {
			topology.Neighbors = append(topology.Neighbors, convertToSHA256Hash(event.Tokens[idx+3]))
		}
		node.logInfo(fmt.Sprint(topology))
		node.sendHTTPEvent(topology, "/api/v1/node-topology", "POST")
	case 1:
		node.logInfo("Init TL event")
		if len(event.Tokens) != 3 {
			node.logError("Invalid tokens count on init TL event")
			return
		}
		equivalent, err := strconv.Atoi(event.Tokens[0])
		if err != nil {
			node.logError("Can't convert equivalent " + event.Tokens[0])
			return
		}
		trustLine := TrustLine{
			Source:      convertToSHA256Hash(event.Tokens[1]),
			Destination: convertToSHA256Hash(event.Tokens[2]),
			Equivalent:  uint32(equivalent)}
		node.logInfo(fmt.Sprint(trustLine))
		node.sendHTTPEvent(trustLine, "/api/v1/trustline", "POST")
	case 2:
		node.logInfo("Close TL event")
		if len(event.Tokens) != 3 {
			node.logError("Invalid tokens count on close TL event")
			return
		}
		trustLine := TrustLine{
			Source:      convertToSHA256Hash(event.Tokens[1]),
			Destination: convertToSHA256Hash(event.Tokens[2])}
		node.logInfo(fmt.Sprint(trustLine))
		node.sendHTTPEvent(trustLine, "/api/v1/trustline", "DELETE")
	case 3:
		node.logInfo("Payment event")
		if len(event.Tokens) < 4 {
			node.logError("Invalid tokens count on payment event")
			return
		}
		equivalent, err := strconv.Atoi(event.Tokens[0])
		if err != nil {
			node.logError("Can't convert equivalent " + event.Tokens[0])
			return
		}
		payment := Payment{
			TransactionUUID: event.Tokens[1],
			Source:          convertToSHA256Hash(event.Tokens[2]),
			Destination:     convertToSHA256Hash(event.Tokens[3]),
			Equivalent:      uint32(equivalent)}
		var paymentPath []string
		for idx := 4; idx < len(event.Tokens); idx++ {
			if event.Tokens[idx] == event.Tokens[3] {
				payment.Paths = append(payment.Paths, paymentPath)
				paymentPath = nil
			} else {
				paymentPath = append(paymentPath, convertToSHA256Hash(event.Tokens[idx]))
			}
		}
		node.logInfo(fmt.Sprint(payment))
		node.sendHTTPEvent(payment, "/api/v1/payment", "POST")
	default:
		node.logError("Unexpected event type " + strconv.Itoa(event.Code))
	}
}

func convertToSHA256Hash(nodeAddress string) string {
	bytesArray := sha256.Sum256([]byte(nodeAddress))
	hash := hex.EncodeToString(bytesArray[:])
	return hash
}

func (node *Node) sendHTTPEvent(data interface{}, requestHeader string, method string) {
	url := fmt.Sprint(conf.Params.Service.URL, requestHeader)
	logger.Info("Try send request: " + url)
	js, err := json.Marshal(data)
	if err != nil {
		logger.Error("Can't marshall data. Details are: " + err.Error())
		return
	}
	logger.Debug("JSON: " + string(js))

	request, err := http.NewRequest(method, url, bytes.NewBuffer(js))
	if err != nil {
		logger.Error("Can't create request. Details: " + err.Error())
		return
	}
	request.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	response, err := client.Do(request)
	if err != nil {
		logger.Error("Can't send request " + err.Error())
		return
	}

	logger.Debug("Server response: " + response.Status)
	response.Body.Close()
}

func (node *Node) logError(message string) {
	logger.Error(node.logHeader() + message)
}

func (node *Node) logInfo(message string) {
	logger.Info(node.logHeader() + message)
}

func (node *Node) logDebug(message string) {
	logger.Debug(node.logHeader() + message)
}

func (node *Node) logHeader() string {
	return "[Node]: "
}
