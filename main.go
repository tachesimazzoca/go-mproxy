package main

import (
	"fmt"
	"log"
	"net"

	"github.com/tachesimazzoca/go-mproxy/smtp"
)

func assertNoError(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func main() {
	lsnr, err := net.Listen("tcp", "localhost:1025")
	assertNoError(err)
	for {
		conn, err := lsnr.Accept()
		assertNoError(err)
		h := smtp.NewSMTPHandler(conn, func(st *smtp.SMTPState) error {
			fmt.Println(st)
			return nil
		})
		go h.Run()
	}
}
