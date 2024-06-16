package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"os"
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
	uniqueIPs = make(map[string]struct{})
	mu        sync.Mutex
)

func pingClient(c net.Conn, err_chan chan bool) {

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			fmt.Println("sending ping")
			result := "ping" + "\n"

			_, err := c.Write([]byte(string(result)))
			if err != nil {
				fmt.Println("Write error")
				fmt.Println(err)
				err_chan <- true
				return
			}
			c.SetDeadline(time.Now().Add(time.Second * 2))
			netData, err := bufio.NewReader(c).ReadString('\n')

			if err != nil {
				fmt.Println("no ping response here")
				fmt.Println(err)
				err_chan <- true
				return
			}

			temp := strings.TrimSpace(string(netData))
			slog.Info(temp)
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
		uniqueIPs[ip] = struct{}{}
	}
	mu.Unlock()

	err_chan := make(chan bool)

	fmt.Printf("Serving %s\n", c.RemoteAddr().String())
	go pingClient(c, err_chan)
	/*
		for {
			netData, err := bufio.NewReader(c).ReadString('\n')
			if err != nil {
				fmt.Println("exiting here")
				fmt.Println(err)
				return
			}

			temp := strings.TrimSpace(string(netData))
			slog.Info(temp)
			if temp == "STOP" {
				break
			}

		}
	*/
	defer c.Close()
	<-err_chan
	fmt.Println("closing this conn")
	mu.Lock()
	delete(uniqueIPs, ip)
	fmt.Println("IP removed:", ip)
	mu.Unlock()
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
		l, err := net.Listen("tcp", "localhost:"+*serverport)
		if err != nil {
			fmt.Println(err)
			return
		}
		defer l.Close()
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

		reply := make([]byte, 1024)
		for range 5 {

			_, err = conn.Read(reply)
			if err != nil {
				fmt.Println("Read from server failed:", err.Error())
				os.Exit(1)
			}
			fmt.Println("reply from server=", strings.TrimSpace(string(reply)))
			if string(bytes.Trim(reply, "\x00")) == "ping\n" {
				// respond to ping
				fmt.Println("Responding to ping")
				_, err = conn.Write([]byte("pong\n"))
				if err != nil {
					println("Write to server failed:", err.Error())
					os.Exit(1)
				}

			}

		}
		for {

		}
		conn.Close()

	}

}
