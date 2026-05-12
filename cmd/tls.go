package cmd

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"os"
	pb "pb"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

func SetClientStreamConn() (err error) {
	hostPort := strings.Join([]string{Host, Port}, ":")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	gClientConn, err = grpc.DialContext(ctx, hostPort,
		grpc.WithInsecure(),
		grpc.WithBlock(),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(MaxMessageSize), grpc.MaxCallSendMsgSize(MaxMessageSize)))

	if err != nil {
		PrintError("ClientSendFiles", err)
		return err
	}

	//defer gClientConn.Close()

	gClient = pb.NewFileTransferClient(gClientConn)
	if gClient == nil {
		return NewError("gClient cannot be empty")
	}

	gClientStream, err = gClient.StreamReceive(context.Background())
	if err != nil {
		PrintError("ClientSendFiles", err)
		return err
	}

	return nil
}

func SetTLSClientStreamConn() (err error) {
	certificate, err := tls.LoadX509KeyPair("cert/client/client.crt", "cert/client/client.key")
	if err != nil {
		FatalError("SetTLSClientStreamConn:tls.LoadX509KeyPair", err)
	}

	certPool := x509.NewCertPool()
	ca, err := os.ReadFile("cert/ca.crt")
	if err != nil {
		FatalError("SetTLSClientStreamConn:os.ReadFile", err)
	}

	if ok := certPool.AppendCertsFromPEM(ca); !ok {
		FatalError("SetTLSClientStreamConn:os.ReadFile", NewError("certPool.AppendCertsFromPEM"))
	}

	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(credentials.NewTLS(
			&tls.Config{
				ServerName:   Host,
				Certificates: []tls.Certificate{certificate},
				RootCAs:      certPool,
			})),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(MaxMessageSize), grpc.MaxCallSendMsgSize(MaxMessageSize)),
	}

	hostPort := strings.Join([]string{Host, Port}, ":")

	gClientConn, err = grpc.Dial(hostPort, opts...)

	if err != nil {
		FatalError("SetTLSClientStreamConn", err)
	}

	//defer gClientConn.Close()

	gClient = pb.NewFileTransferClient(gClientConn)
	if gClient == nil {
		return NewError("gClient cannot be empty")
	}

	gClientStream, err = gClient.StreamReceive(context.Background())
	if err != nil {
		PrintError("ClientSendFiles", err)
		return err
	}

	return nil
}
