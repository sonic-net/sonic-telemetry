// The dialout_client_cli program implements the telemetry publish client.
package main

import (
	"crypto/tls"
	"flag"
	dc "github.com/Azure/sonic-telemetry/dialout/dialout_client"
	log "github.com/golang/glog"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"golang.org/x/net/context"
	"os"
	"os/signal"
	"time"
)

var (
	clientCfg = dc.ClientConfig{
		RetryInterval:  30 * time.Second,
		Encoding:       gpb.Encoding_JSON_IETF,
		Unidirectional: true,
	}

	tlsCfg = tls.Config{}

	tlsDisable bool
)

func init() {
	flag.StringVar(&tlsCfg.ServerName, "server_name", "", "When set, use this hostname to verify server certificate during TLS handshake.")
	flag.BoolVar(&tlsCfg.InsecureSkipVerify, "skip_verify", false, "When set, client will not verify the server certificate during TLS handshake.")
	flag.BoolVar(&tlsDisable, "insecure", false, "Without TLS, only for testing")
	flag.DurationVar(&clientCfg.RetryInterval, "retry_interval", 30*time.Second, "Interval at which client tries to reconnect to destination servers")
	flag.BoolVar(&clientCfg.Unidirectional, "unidirectional", true, "No repesponse from server is expected")
}

func main() {
	flag.Parse()
	ctx, cancel := context.WithCancel(context.Background())
	// Terminate on Ctrl+C
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt)
		<-c
		cancel()
	}()
	log.V(1).Infof("Starting telemetry publish client")
	if !tlsDisable {
		clientCfg.TLS = &tlsCfg
		log.V(1).Infof("TLS enable")
	}
	err := dc.DialOutRun(ctx, &clientCfg)
	log.V(1).Infof("Exiting telemetry publish client: %v", err)
	log.Flush()
}
