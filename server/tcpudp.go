/*********************************************************************************
*      File Name     :     tcpudp.go
*      Created By    :     xbg@maqi.me
*      Creation Date :     [2015-12-24 14:37]
*      Last Modified :     [AUTO_UPDATE_BEFORE_SAVE]
*      Description   :
*      Copyright     :     2015 xbg@maqi.me
*      License       :     Licensed under the Apache License, Version 2.0
**********************************************************************************/
package server

import (
	"bufio"
	"bytes"
	"echo/bpool"
	"echo/libsodium"
	"echo/netstring"
	"io"
	"log"
	"net"
	"os"
	"time"
)

var homeip string = ":0"

func tcpListener(addr string) (*net.TCPListener, error) {
	tcpaddr, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		return nil, err
	}
	return net.ListenTCP("tcp", tcpaddr)
}

func TcpServe(addr string, encryptKey []byte) error {
	tcplistener, err := tcpListener(addr)
	log.SetOutput(os.Stdout)
	if err != nil {
		return err
	}
	bpool := bpool.NewSizedBufferPool(20, 4096)

	for {
		tcpconn, err := tcplistener.AcceptTCP()
		if err != nil {
			log.Printf("AcceptTCP error : %v\n", err)
			continue
		}
		buffer := bpool.Get()

		go handleTCPConn(tcpconn, encryptKey, buffer, bpool)
	}
}

func handleTCPConn(tcpconn *net.TCPConn, encryptKey []byte, buffer *bytes.Buffer, bpool *bpool.SizedBufferPool) {
	defer tcpconn.Close()
	defer bpool.Put(buffer)
	var receiveData []byte
	//tcpconn need read all data in 20 second ,otherwise Timeout() will be true
	tcpconn.SetReadDeadline(time.Now().Add(time.Duration(20) * time.Second))
	bufReader := bufio.NewReader(tcpconn)
	for {
		rData, err := bufReader.ReadBytes(',')
		if err != nil {
			if err == io.EOF {
				log.Printf("TCPConn Read error\n")
				return
			}
			buffer.Write(rData)
			continue
		}

		buffer.Write(rData)

		receiveData, err = netstring.Unmarshall(buffer.Bytes())
		if err != nil {
			if err == netstring.ErrNsLenNotEqaulOrgLen {
				continue
			} else {
				log.Printf("netstring unmarshall error : %v\n", err)
				return
			}
		}

		break
	}

	_, err := libsodium.DecryptData(encryptKey, receiveData)
	if err != nil {
		log.Printf("tcp DecryptData error : %v\n", err)
		return
	}

	tcpconn.SetWriteDeadline(time.Now().Add(time.Duration(20) * time.Second))
	_, err = tcpconn.Write(netstring.Marshall([]byte(homeip)))
	if err != nil {
		log.Printf("tcpconn error : %v\n", err)
	}
}

func UdpServe(addr string, key []byte) error {
	udpaddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return err
	}

	udpconn, err := net.ListenUDP("udp", udpaddr)
	if err != nil {
		log.Printf("udpServer error : %v\n", err)
		return err
	}

	var receiveData = make([]byte, 1024)
	for {
		handleUDPConn(udpconn, key, receiveData)
	}
}

func handleUDPConn(udpconn *net.UDPConn, key []byte, receiveData []byte) {
	//udpconn.SetReadDeadline(time.Now().Add(time.Duration(20) * time.Second))
	receiveDatalen, addr, err := udpconn.ReadFrom(receiveData)
	if err != nil {
		log.Printf("udpconn ReadFrom err : %v\n", err)
	}

	receiveData, err = netstring.Unmarshall(receiveData[:receiveDatalen])
	if err != nil {
		log.Printf("netstring unmarshall error : %v\n", err)
		return
	}

	_, err = libsodium.DecryptData(key, receiveData)
	if err != nil {
		log.Printf("udp DecryptData error : %v\n", err)
		return
	}

	homeip = addr.String()
}
