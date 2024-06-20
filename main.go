package main

import (
	"bufio"
	"flag"
	"fmt"
	"log/slog"
	"math/rand"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Client server architecture
// command line option for client
// or server server gather clients
// make sure they are still alive and
// send commands for them to run to cluster work
// between many clients
// config file for what ips the clients are ?

var (
	uniqueIPs = make(map[string]clientStatus)
	mu        sync.Mutex
	taskQueue = make(chan string, 10) // Task queue for managing tasks
	working   = false
)

type clientStatus struct {
	working bool
}

func sendCommand(c net.Conn, command string) error {
	_, err := c.Write([]byte(command))
	return err
}

func handleCluster(c net.Conn, errChan chan bool) {
	// server side handling of client cluster connection
	go acceptTask(c, errChan)
	go readResponses(c, errChan)
}

func acceptTask(c net.Conn, errChan chan bool) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			fmt.Println("Checking if time to send out work")

			// Check for available tasks and send to client
			select {
			case task := <-taskQueue:
				// check if remote addr is "working"

				mu.Lock()

				isWorking := uniqueIPs[c.RemoteAddr().String()].working

				if isWorking {
					fmt.Println("this connection is working sending back task", task)
					taskQueue <- task
					mu.Unlock()
					continue
				}
				if status, exists := uniqueIPs[c.RemoteAddr().String()]; exists {
					status.working = true
					uniqueIPs[c.RemoteAddr().String()] = status
				}
				mu.Unlock()

				fmt.Println("Assigning task:", task, "to ", c.RemoteAddr().String())
				if err := sendCommand(c, task+"\n"); err != nil {
					fmt.Println("Error sending task:", err)
					errChan <- true
					return
				}

			default:
				// No tasks available
			}
		}
	}
}

func readResponses(c net.Conn, errChan chan bool) {
	reader := bufio.NewReader(c)
	for {
		//c.SetReadDeadline(time.Now().Add(10 * time.Second))
		netData, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println("No response from client:", err)
			errChan <- true
			return
		}

		response := strings.TrimSpace(netData)
		fmt.Println("Received response:", response, "from", c.RemoteAddr().String())
		if strings.Contains(response, "completed") {
			fmt.Println("Setting working for", c.RemoteAddr().String(), " connection to false")
			mu.Lock()
			if status, exists := uniqueIPs[c.RemoteAddr().String()]; exists {
				status.working = false
				uniqueIPs[c.RemoteAddr().String()] = status
			}
			mu.Unlock()
		}
		c.SetReadDeadline(time.Time{}) // Clear deadline

		if response == "done" {
			fmt.Printf("Client %s completed a task\n", c.RemoteAddr().String())
		}
	}
}

// inside here need to handle pinging every once in a while,
// if no response then should consider the connection dead
func handleConnection(c net.Conn) {

	mu.Lock()

	ip := c.RemoteAddr().String()
	if _, exists := uniqueIPs[ip]; !exists {
		fmt.Println("New unique IP connected:", ip)
		uniqueIPs[ip] = clientStatus{working: false}
	}
	mu.Unlock()

	err_chan := make(chan bool)

	fmt.Printf("Serving %s\n", c.RemoteAddr().String())
	go handleCluster(c, err_chan)

	defer c.Close()
	<-err_chan
	fmt.Println("closing this conn")
	mu.Lock()
	delete(uniqueIPs, ip)
	fmt.Println("IP removed:", ip)
	mu.Unlock()
}

func gen_tasks() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:

			num := rand.Int() % 100
			fmt.Printf("sending out task_%d\n", num+1)
			select {
			case taskQueue <- fmt.Sprintf("task_%d", num+1):
			default:
				fmt.Println("Task Queue Channel full. Discarding value")
			}

		}
	}

}

func getTaskNum(input string) (int, error) {
	// Split the string by the underscore
	parts := strings.Split(input, "_")
	if len(parts) != 2 {
		return 0, fmt.Errorf("invalid input format")
	}

	// Convert the second part to an integer
	parts[1] = strings.TrimRight(parts[1], "\n")
	number, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, fmt.Errorf("invalid number: %v", err)
	}

	return number, nil
}

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, nil)))
	serverport := flag.String("serverport", "", "port of the command and control server")
	serverip := flag.String("serverip", "", "ip of command and control server")
	server := flag.Bool("server", false, "bool controlling whether this process should be client or server")
	flag.Parse()
	fmt.Println("server:", *server)
	fmt.Println("serverip:", *serverip)
	fmt.Println("serverport:", *serverport)
	if *server {
		// host tcp server
		l, err := net.Listen("tcp", "0.0.0.0:"+*serverport)
		if err != nil {
			fmt.Println(err)
			return
		}
		defer l.Close()

		go gen_tasks()

		for {
			c, err := l.Accept()
			if err != nil {
				fmt.Println(err)
				return
			}

			fmt.Println("Connected ips")
			for k, _ := range uniqueIPs {
				fmt.Printf("ip %s\n", k)
			}
			go handleConnection(c)
		}

	} else {
		// check that a valid ip and port
		// was provided to connect to the server
		serverAddr := *serverip + ":" + *serverport
		tcpAddr, err := net.ResolveTCPAddr("tcp", serverAddr)
		if err != nil {
			println("ResolveTCPAddr failed:", err.Error())
			os.Exit(1)
		}

		conn, err := net.DialTCP("tcp", nil, tcpAddr)
		if err != nil {
			println("Dial failed:", err.Error())
			os.Exit(1)
		}

		_, err = conn.Write([]byte("hello ready to work\n"))
		if err != nil {
			println("Write to server failed:", err.Error())
			os.Exit(1)
		}

		//reply := make([]byte, 1024)
		reader := bufio.NewReader(conn)
		for {
			reply, err := reader.ReadString('\n')
			if err != nil {
				fmt.Println("Read from server failed:", err.Error())
				os.Exit(1)
			}
			trimmedReply := reply //string(bytes.Trim(reply, "\x00"))
			fmt.Println("Got from server", reply)

			if strings.Contains(trimmedReply, "task") {
				task_num, err := getTaskNum(reply)
				if err != nil {
					println("Get task num failed:", err.Error())
				}
				fmt.Println("Got number", task_num)
				working = true

				time.Sleep(time.Duration(time.Second * 20))
				fmt.Println("Completed task ")
				working = false
				message_to_server := fmt.Sprintf("completed task %d\n", task_num)
				fmt.Println("Sending message to server ", message_to_server)
				_, err = conn.Write([]byte(message_to_server))
				if err != nil {
					println("Write to server failed", err.Error())
					os.Exit(1)
				}
			}
			slog.Info(trimmedReply)

		}

		conn.Close()

	}

}
