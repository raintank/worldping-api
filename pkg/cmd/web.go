// Copyright 2014 Unknwon
// Copyright 2014 Torkel Ödegaard

package cmd

import (
	"crypto/tls"
	_ "expvar"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/Unknwon/macaron"
	"github.com/macaron-contrib/toolbox"

	"github.com/raintank/worldping-api/pkg/api"
	"github.com/raintank/worldping-api/pkg/log"
	"github.com/raintank/worldping-api/pkg/setting"
)

func newMacaron() *macaron.Macaron {
	macaron.Env = setting.Env
	m := macaron.Classic()
	m.Use(toolbox.Toolboxer(m))
	m.Use(func(ctx *macaron.Context) {
		if ctx.Req.URL.Path == "/debug/vars" {
			http.DefaultServeMux.ServeHTTP(ctx.Resp, ctx.Req.Request)
		}
	})

	return m
}

func StartServer(notifyShutdown chan struct{}) {
	var err error
	m := newMacaron()
	api.Register(m)

	listenAddr := fmt.Sprintf("%s:%s", setting.HttpAddr, setting.HttpPort)
	log.Info("Listen: %v://%s%s", setting.Protocol, listenAddr, setting.AppSubUrl)

	// define our own listner so we can call Close on it
	l, err := net.Listen("tcp", listenAddr)
	if err != nil {
		log.Fatal(4, err.Error())
	}
	go handleShutdown(notifyShutdown, l)
	srv := http.Server{
		Addr:    listenAddr,
		Handler: m,
	}
	if setting.Protocol == setting.HTTPS {
		cert, err := tls.LoadX509KeyPair(setting.CertFile, setting.KeyFile)
		if err != nil {
			log.Fatal(4, "Fail to start server: %v", err)
		}
		srv.TLSConfig = &tls.Config{
			Certificates: []tls.Certificate{cert},
			NextProtos:   []string{"http/1.1"},
		}
		tlsListener := tls.NewListener(tcpKeepAliveListener{l.(*net.TCPListener)}, srv.TLSConfig)
		err = srv.Serve(tlsListener)
	} else {
		err = srv.Serve(tcpKeepAliveListener{l.(*net.TCPListener)})
	}

	if err != nil {
		log.Info(err.Error())
	}
}

func handleShutdown(notifyShutdown chan struct{}, l net.Listener) {
	<-notifyShutdown
	log.Info("shutdown started.")
	l.Close()
}

type tcpKeepAliveListener struct {
	*net.TCPListener
}

func (ln tcpKeepAliveListener) Accept() (c net.Conn, err error) {
	tc, err := ln.AcceptTCP()
	if err != nil {
		return
	}
	tc.SetKeepAlive(true)
	tc.SetKeepAlivePeriod(3 * time.Minute)
	return tc, nil
}
