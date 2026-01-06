package grpc

import (
	"context"
	"net"

	pb "github.com/dmehra2102/Order-Management-System/internal/inventory/infrastructure/grpc/proto"
	"google.golang.org/grpc"
)

type Server struct {
	pb.UnimplementedInventoryServiceServer
}

func NewServer() *Server { return &Server{} }

func (s *Server) CheckStock(ctx context.Context, req *pb.CheckStockRequest) (*pb.CheckStockResponse, error) {
	// Mocking: always available
	return &pb.CheckStockResponse{Available: true}, nil
}

func Run(addr string, srv *Server) (*grpc.Server, error) {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	gs := grpc.NewServer()
	pb.RegisterInventoryServiceServer(gs, srv)
	go func() {
		_ = gs.Serve(lis)
	}()
	return gs, nil
}
