package main

import (
	"os"
	"fmt"
	"net"
	"net/http"
	"bufio"
	"io/ioutil"
	"strings"
	"bytes"
	"encoding/json"
	"path"
)

type Process struct {
	Clients map[string]bool
	Bundle  []byte
	URL     string
}

type Request struct {
	URL    string
	Output chan []byte
	Source string
}

// Configuration struct to be replaced by a decoded JSON file's contents.
var Configuration = struct {
	PortNumber     string
	EdgeServer     string
	BridgeServer   string
	ErrorMsg       string
	PleaseWaitPage string
} {}

func pleaseWait(url string) []byte {
	content, _ := ioutil.ReadFile(Configuration.PleaseWaitPage)
	return bytes.Replace(content, []byte("{{REDIRECT}}"), []byte(url), 1)
}

// Have the Transport Server create a new bundle
func requestNewBundle(url string, reportCompletion chan Process) bool {
	remoteAddr, _ := net.ResolveTCPAddr("tcp", Configuration.BridgeServer)
	conn, err := net.DialTCP("tcp", nil, remoteAddr)
	readyMSG := []byte("READY\n")
	errorMSG := []byte("ERROR expected message COMPLETE\n")
	if err != nil {
		fmt.Println("Could not establish connection to bridge server at " + Configuration.BridgeServer)
		return false // Failed to get new bundle
	}
	reader := bufio.NewReader(conn)
	conn.Write([]byte("BUNDLE " + url + "\n"))
	result, _ := reader.ReadString('\n')
	if !strings.HasPrefix(result, "COMPLETE") {
		if strings.HasPrefix(result, "ERROR") {
			fmt.Println("!!! Got error message " + result[strings.Index(result, " "):])
			// Report process completion so that requests do not sit around in memory.
			// Serve the error as the bundle so it reaches the user.
			reportCompletion <- Process { Clients: nil, Bundle: []byte(result), URL: url }
		} else {
			fmt.Println("Bridge server not adhering to protocol.")
			fmt.Println("In response to BUNDLE, sent " + result)
			conn.Write(errorMSG)
			// Report process completion so that requests do not sit around in memory.
			reportCompletion <- Process { Clients: nil, Bundle: errorMSG, URL: url }
		}
		conn.Close()
		return false
	}
	conn.Write(readyMSG)
	bundle, _ := ioutil.ReadAll(reader)
	reportCompletion <- Process { Clients: nil, Bundle: bundle, URL: url }
	conn.Close()
	return true
}

// Check if a bundle has already been cached and, if so, write it to the ResponseWriter
func readFromCache(url string, reportCompletion chan Process) bool {
	remoteAddr, _ := net.ResolveTCPAddr("tcp", Configuration.EdgeServer)
	conn, err := net.DialTCP("tcp", nil, remoteAddr)
	okayMSG := []byte("OKAY\n")
	readyMSG := []byte("READY\n")
	errorMSG := []byte("ERROR expected message RESULT (not) found\n")
	if err != nil {
		fmt.Println("Could not establish connection to edge server at " + Configuration.BridgeServer)
		return false // Failed to lookup
	}
	reader := bufio.NewReader(conn)
	conn.Write([]byte("LOOKUP " + url + "\n"))
	result, _ := reader.ReadString('\n')
	if !strings.HasPrefix(result, "RESULT") {
		if strings.HasPrefix(result, "ERROR") {
			fmt.Print("!!! Got error message " + result[strings.Index(result, " ") + 1:])
		} else {
			fmt.Println("Edge server not adhering to protocol.")
			fmt.Println("In response to LOOKUP, sent " + result)
			conn.Write(errorMSG)
		}
		conn.Close()
		return false
	}
	if strings.HasSuffix(result, "not found\n") {
		fmt.Println("No bundle found in cache")
		conn.Write(okayMSG)
		conn.Close()
		return false
	} else if strings.HasSuffix(result, "found\n") {
		conn.Write(readyMSG)
		bundle, _ := ioutil.ReadAll(reader)
		if (bytes.HasPrefix(bundle, []byte("ERROR"))) {
			error := string(bundle)
			fmt.Println("!!! Got error instead of bundle: " + error[strings.Index(error, " "):])
			conn.Close()
			return false
		} else {
			reportCompletion <- Process { Clients: nil, Bundle: bundle, URL: url }
			conn.Close()
			return true
		}
	}
	fmt.Println("Unrecognized RESULT status received from edge server")
	conn.Write(errorMSG)
	conn.Close()
	return false
}

func getBundle(url string, reportCompletion chan Process) {
	foundInCache := readFromCache(url, reportCompletion)
	if !foundInCache {
		fmt.Println("Did not find a bundle for " + url + " in cache")
		producedBundle := requestNewBundle(url, reportCompletion)
		if !producedBundle {
			fmt.Println("Error; Could not produce new bundle")
		} else {
			fmt.Println("Created a new bundle for " + url)
		}
	} else {
		fmt.Println("Found bundle for " + url + " in cache")
	}
}

func issueBundles(requests chan Request) {
	processes := make(map[string]Process)
	// Have the cache lookup and transport communicator routines commuicate when
	// new bundles have been found or produced through the this channel
	finishedProcesses := make(chan Process)
	for {
		select {
		case request := <-requests:
			_, exists := processes[request.URL]
			if exists { // We have already started processing a lookup or bundling for this URL
				fmt.Println("Existing process to bundle " + request.URL + " found")
				if len(processes[request.URL].Bundle) > 0 { //The bundle has been prepared
					request.Output <- processes[request.URL].Bundle
					_, hasRequested := processes[request.URL].Clients[request.Source]
					if hasRequested {
						// Remove the source of the request from the set of clients
						// so we can get closer to removing the bundle from memory
						delete(processes[request.URL].Clients, request.Source)
						fmt.Println("Removed " + request.Source + " as a client")
					}
					if len(processes[request.URL].Clients) == 0 {
						// Remove the process from memory since no one is waiting on it
						fmt.Println("Removing process for " + request.URL)
						delete(processes, request.URL)
					}
				} else {
					fmt.Println("Serving please wait page to " + request.Source)
					request.Output <- pleaseWait(request.URL)
					_, hasRequested := processes[request.URL].Clients[request.Source]
					if !hasRequested {
						// Add the new source to the set of clients waiting for the bundle
						processes[request.URL].Clients[request.Source] = true
					}
				}
			} else {
				// Start a new process for the requested URL's bundle
				fmt.Println("Creating a new process to get the bundle for " + request.URL)
				firstClient := make(map[string]bool)
				firstClient[request.Source] = true
				processes[request.URL] = Process { Clients: firstClient, Bundle: []byte(""), URL: request.URL }
				go getBundle(request.URL, finishedProcesses)
				request.Output <- pleaseWait(request.URL)
			}
		case finished := <-finishedProcesses:
			// When either a successful cache lookup completes or a new bundle is produced,
			// store the bundle's contents in an existing process to be fetched by clients
			fmt.Println("Finished retrieving a bundle for " + finished.URL)
			_, stillWorking := processes[finished.URL]
			if stillWorking {
				// Cannot assign directly to a map's struct value so use this workaround
				tempProc := processes[finished.URL]
				tempProc.Bundle = finished.Bundle
				processes[finished.URL] = tempProc
			}
		}
	}
}

func makeProxyHandler(toDealer chan Request) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		// Leave it to the bundle dealer to serve the bundle or please wait page.
		// This avoids duplicating bundles in memory through a channel.
		fmt.Println("Requesting bundle for " + r.RequestURI + " on behalf of " + r.RemoteAddr)
		out := make(chan []byte)
		host, _, _ := net.SplitHostPort(r.RemoteAddr)
		toDealer <- Request { r.RequestURI, out, host }
		page := <-out
		w.Write(page)
	}
}

// Create an HTTP proxy server to listen on port 3090
func main() {
	// Read the configuration JSON file into the global Configuration
	configPath := path.Join("..", "config", "client.json")
	file, _ := os.Open(configPath)
	decoder := json.NewDecoder(file)
	err := decoder.Decode(&Configuration)
	if err != nil {
		fmt.Println("Could not read configuration file at " + configPath + "\nExiting.")
		return
	}
	toBundleDealer := make(chan Request)
	// Read requests for bundles for given URLs from toBundleDealer, serve through frBundleDealer
	go issueBundles(toBundleDealer)
	http.HandleFunc("/", makeProxyHandler(toBundleDealer))
	fmt.Println("CeNo proxy server listening at http://localhost" + Configuration.PortNumber)
	http.ListenAndServe(Configuration.PortNumber, nil)
}