package grpc

import (
	"context"
	"log/slog"

	pb "github.com/dmehra2102/Order-Management-System/internal/inventory/infrastructure/grpc/proto"
	"github.com/dmehra2102/Order-Management-System/internal/order/domain"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type InventoryClient struct {
	log *slog.Logger
	cc  pb.InventoryServiceClient
}

func NewInventoryClient(log *slog.Logger, addr string) (*InventoryClient, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	return &InventoryClient{
		log: log,
		cc:  pb.NewInventoryServiceClient(conn),
	}, nil
}

func (c *InventoryClient) CheckStock(ctx context.Context, items []domain.OrderItem) (bool, error) {
	req := &pb.CheckStockRequest{Items: make([]*pb.Item, 0, len(items))}

	for _, item := range items {
		req.Items = append(req.Items, &pb.Item{Sku: item.SKU, Quantity: int32(item.Quantity)})
	}
	resp, err := c.cc.CheckStock(ctx, req)
	if err != nil {
		return false, err
	}
	return resp.Available, nil
}
