package main

import (
	"encoding/gob"
	"fmt"
	"github.com/songgao/water"
	"io"
	"log"
	"net"
	"os"
	"sync"
	"time"
)

var (
	encoder *gob.Encoder
	decoder *gob.Decoder

	wg sync.WaitGroup
)

type ClientData struct {
	DestAddr string
	Data     []byte
}

const BUFFERSIZE = 1528

func main() {
	log.Println("Client start")

	if len(os.Args) < 3 {
		log.Println("Please spicify server address and port")
		return
	}

	server_addr := fmt.Sprintf("%s:%s", os.Args[1], os.Args[2])

	server_conn, err := net.DialTimeout("tcp", server_addr, 3*time.Second)
	if err != nil {
		log.Println("cannot connect to server:", err)
		return
	}

	iface, err := water.NewTAP("")
	if err != nil {
		log.Println("Create new TAP interface failed:", err)
		return
	}

	encoder = gob.NewEncoder(server_conn)
	decoder = gob.NewDecoder(server_conn)

	go ConnToIface(server_conn, iface)
	go IfaceToConn(server_conn, iface)

	wg.Add(2)
	wg.Wait()
}

func ConnToIface(conn net.Conn, iface *water.Interface) {
	defer wg.Done()
	client_data := new(ClientData)
	for {

		err := decoder.Decode(client_data)
		if err == io.EOF {
			log.Println("Client connection close")
			conn.Close()
			iface.Close()
			break
		}

		if err != nil {
			log.Println("decode data failed:", err)
			conn.Close()
			iface.Close()
			break
		}

		_, err = iface.Write(client_data.Data)

		if err != nil {
			log.Println("Write data to NIC failed:", err)
			conn.Close()
			iface.Close()
			break
		}
	}
}

func IfaceToConn(conn net.Conn, iface *water.Interface) {
	defer wg.Done()
	buffer := make([]byte, BUFFERSIZE)
	client_data := new(ClientData)
	for {
		n, err := iface.Read(buffer)
		log.Printf("read %d bytes data from NIC\n", n)
		if err != nil {
			log.Println("Read from NIC failed:", err)
			iface.Close()
			conn.Close()
			break
		}

		client_data.DestAddr = "client"
		client_data.Data = buffer[:n]

		err = encoder.Encode(client_data)
		if err != nil {
			log.Println("Send data to client failed:", err)
			iface.Close()
			conn.Close()
			break
		}
	}
}
