package main

import (
	"bufio"
	"flag"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
)

// Client server architecture
// command line option for client
// or server server gather clients
// make sure they are still alive and
// send commands for them to run to cluster work
// between many clients
// config file for what ips the clients are ?

func handleConnection(c net.Conn) {
	fmt.Printf("Serving %s\n", c.RemoteAddr().String())
	for {
		netData, err := bufio.NewReader(c).ReadString('\n')
		if err != nil {
			fmt.Println(err)
			return
		}

		temp := strings.TrimSpace(string(netData))
		fmt.Println("Got message ", temp)
		if temp == "STOP" {
			break
		}

		result := strconv.Itoa(122) + "\n"
		c.Write([]byte(string(result)))
	}
	c.Close()
}

func main() {
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

		_, err = conn.Write([]byte("test\n"))
		if err != nil {
			println("Write to server failed:", err.Error())
			os.Exit(1)
		}

		reply := make([]byte, 1024)

		_, err = conn.Read(reply)
		if err != nil {
			fmt.Println("Write to server failed:", err.Error())
			os.Exit(1)
		}
		fmt.Println("reply from server=", string(reply))
		conn.Close()

	}

}
