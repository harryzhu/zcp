package cmd

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net"
	"os"
	pb "pb"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	//
	//gClientConn        *grpc.ClientConn
	maxGrpcMessageSize int = 4 << 30
	//
	tlsServerCert string = "cert/server/server.crt"
	tlsServerKey  string = "cert/server/server.key"
	//
	tlsClientCert string = "cert/client/client.crt"
	tlsClientKey  string = "cert/client/client.key"
	//
	tlsCA string = "cert/ca.crt"
)

func GetClientStream() pb.FileTransfer_StreamReceiveClient {
	client := GetClient()

	clientStream, err := client.StreamReceive(context.Background())
	FatalError("GetClientStream", err)

	return clientStream
}

func GetClient() pb.FileTransferClient {
	var client pb.FileTransferClient
	if IsWithTLS {
		client = _getTLSClient()
	} else {
		client = _getClient()
	}
	return client
}

func _getClient() pb.FileTransferClient {
	gClientConn := _buildGrpcClientConn()
	client := pb.NewFileTransferClient(gClientConn)
	if client == nil {
		FatalError("GetClient", NewError("gClient cannot be empty"))
	}

	return client
}

func _getTLSClient() pb.FileTransferClient {
	gClientConn := _buildGrpcTLSClientConn()
	client := pb.NewFileTransferClient(gClientConn)
	if client == nil {
		FatalError("GetTLSClient", NewError("gTLSClient cannot be empty"))
	}

	return client
}

func _buildGrpcClientConn() *grpc.ClientConn {
	hostPort := strings.Join([]string{Host, Port}, ":")

	clientConn, err := grpc.NewClient(
		hostPort,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(maxGrpcMessageSize),
			grpc.MaxCallSendMsgSize(maxGrpcMessageSize),
		),
	)
	FatalError("_buildGrpcClientConn", err)

	return clientConn
}

func _buildGrpcTLSClientConn() *grpc.ClientConn {
	hostPort := strings.Join([]string{Host, Port}, ":")

	certificate, err := tls.LoadX509KeyPair(tlsClientCert, tlsClientKey)
	FatalError("gClientConn: init:tls.LoadX509KeyPair", err)

	certPool := x509.NewCertPool()
	ca, err := os.ReadFile(tlsCA)
	FatalError("gClientConn: init:os.ReadFile", err)

	if ok := certPool.AppendCertsFromPEM(ca); !ok {
		FatalError("gClientConn: init:os.ReadFile", NewError("certPool.AppendCertsFromPEM"))
	}

	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(credentials.NewTLS(
			&tls.Config{
				ServerName:   Host,
				Certificates: []tls.Certificate{certificate},
				RootCAs:      certPool,
			})),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(maxGrpcMessageSize),
			grpc.MaxCallSendMsgSize(maxGrpcMessageSize)),
	}

	//clientConn, err := grpc.Dial(hostPort, opts...)
	clientConn, err := grpc.NewClient(
		hostPort,
		opts...,
	)
	FatalError("gClientConn: init", err)

	return clientConn
}

func StartFileTransferServer() {
	hostPort := strings.Join([]string{Host, Port}, ":")
	listening, err := net.Listen("tcp", hostPort)
	if err != nil {
		FatalError("StartFileTransferServer ", err)
	} else {
		PrintlnInfo("purple", "Endpoint RPC ", hostPort)
	}

	grpcServerFileTransfer := grpc.NewServer(
		grpc.MaxRecvMsgSize(maxGrpcMessageSize),
		grpc.MaxSendMsgSize(maxGrpcMessageSize))

	pb.RegisterFileTransferServer(grpcServerFileTransfer, &FileTransferService{})

	grpcServerFileTransfer.Serve(listening)
}

func StartTLSFileTransferServer() {
	certificate, err := tls.LoadX509KeyPair(tlsServerCert, tlsServerKey)
	if err != nil {
		FatalError("StartTLSFileTransferServer:tls.LoadX509KeyPair", err)
	}

	certPool := x509.NewCertPool()
	ca, err := os.ReadFile(tlsCA)
	if err != nil {
		FatalError("StartTLSFileTransferServer:os.ReadFile", err)
	}

	if ok := certPool.AppendCertsFromPEM(ca); !ok {
		FatalError("StartTLSFileTransferServer:os.ReadFile", NewError("certPool.AppendCertsFromPEM"))
	}

	opts := []grpc.ServerOption{
		grpc.Creds(credentials.NewTLS(&tls.Config{
			ClientAuth:   tls.RequireAndVerifyClientCert,
			Certificates: []tls.Certificate{certificate},
			ClientCAs:    certPool,
		},
		)),
		grpc.MaxRecvMsgSize(maxGrpcMessageSize),
		grpc.MaxSendMsgSize(maxGrpcMessageSize),
	}

	hostPort := strings.Join([]string{Host, Port}, ":")
	listening, err := net.Listen("tcp", hostPort)
	if err != nil {
		FatalError("StartTLSFileTransferServer", err)
	} else {
		PrintlnInfo("green", "Endpoint RPC: ", hostPort)
	}

	grpcServerFileTransfer := grpc.NewServer(opts...)

	pb.RegisterFileTransferServer(grpcServerFileTransfer, &FileTransferService{})

	grpcServerFileTransfer.Serve(listening)
}
