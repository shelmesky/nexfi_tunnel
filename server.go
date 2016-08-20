package main

import (
	"encoding/gob"
	"github.com/songgao/water"
	//"github.com/songgao/water/waterutil"
	"io"
	"log"
	"net"
	"sync"
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
const SERVER_ADDR = "0.0.0.0:9199"

func main() {
	log.Println("Start server")

	server_sock, err := net.Listen("tcp", SERVER_ADDR)

	if err != nil {
		log.Println("can not listen on tcp:", err)
		return
	}

	log.Println("Server listen:", SERVER_ADDR)

	for {
		conn, err := server_sock.Accept()
		if err != nil {
			continue
		}

		encoder = gob.NewEncoder(conn)
		decoder = gob.NewDecoder(conn)

		iface, err := water.NewTAP("")

		go ConnToIface(conn, iface)
		go IfaceToConn(conn, iface)
		wg.Add(2)
	}

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
			break
		}

		if err != nil {
			log.Println("decode data failed:", err)
			conn.Close()
			break
		}

		_, err = iface.Write(client_data.Data)

		if err != nil {
			log.Println("Write data to NIC failed:", err)
			conn.Close()
			iface.Close()
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
