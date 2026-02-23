package client

import (
	"context"
	"fmt"
	"net"
	"os"
	"testing"

	pb "github.com/stain-win/gaia/libs/go/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

// mockGaiaClientServer is a mock of the GaiaClientServer interface for testing.
type mockGaiaClientServer struct {
	pb.UnimplementedGaiaClientServer // Embed for forward compatibility
	GetSecretFunc                    func(ctx context.Context, in *pb.GetSecretRequest) (*pb.Secret, error)
	ListSecretsFunc                  func(ctx context.Context, in *pb.ClientListSecretsRequest) (*pb.ListSecretsResponse, error)
}

func (m *mockGaiaClientServer) GetSecret(ctx context.Context, in *pb.GetSecretRequest) (*pb.Secret, error) {
	if m.GetSecretFunc != nil {
		return m.GetSecretFunc(ctx, in)
	}
	return nil, fmt.Errorf("GetSecretFunc not implemented")
}

func (m *mockGaiaClientServer) ListSecrets(ctx context.Context, in *pb.ClientListSecretsRequest) (*pb.ListSecretsResponse, error) {
	if m.ListSecretsFunc != nil {
		return m.ListSecretsFunc(ctx, in)
	}
	return nil, fmt.Errorf("ListSecretsFunc not implemented")
}

// startTestServer starts a mock gRPC server for testing purposes.
func startTestServer(mock pb.GaiaClientServer) (*grpc.ClientConn, func()) {
	lis := bufconn.Listen(1024 * 1024)
	s := grpc.NewServer()
	pb.RegisterGaiaClientServer(s, mock)

	go func() {
		if err := s.Serve(lis); err != nil {
			panic(fmt.Sprintf("Server exited with error: %v", err))
		}
	}()

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return lis.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		panic(fmt.Sprintf("Failed to dial bufnet: %v", err))
	}

	return conn, func() {
		s.Stop()
		conn.Close()
	}
}

func TestClient(t *testing.T) {
	// Setup a mock server
	// Setup a mock server
	mockServer := &mockGaiaClientServer{
		ListSecretsFunc: func(ctx context.Context, in *pb.ClientListSecretsRequest) (*pb.ListSecretsResponse, error) {
			return &pb.ListSecretsResponse{}, nil
		},
	}
	conn, cleanup := startTestServer(mockServer)
	defer cleanup()

	// We're bypassing the actual connection logic in NewClient for this test setup
	// because we want to use the bufconn-backed connection.

	client := &Client{
		conn: conn,
		// client: pb.NewGaiaClientClient(conn), // This was in original, but we need to check if NewGaiaClientClient is needed or if we can just assign the mock?
		// Wait, pb.NewGaiaClientClient creates a client stub that sends requests over the conn.
		// The conn is connected to the mock server.
		// So yes, we need NewGaiaClientClient.
		client: pb.NewGaiaClientClient(conn),
	}

	t.Run("GetSecret", func(t *testing.T) {
		t.Run("Success", func(t *testing.T) {
			mockServer.GetSecretFunc = func(ctx context.Context, in *pb.GetSecretRequest) (*pb.Secret, error) {
				if in.Namespace == "test-ns" && in.Id == "test-id" {
					return &pb.Secret{Id: "test-id", Value: "test-value"}, nil
				}
				return nil, fmt.Errorf("secret not found")
			}

			value, err := client.GetSecret(context.Background(), "test-ns", "test-id")
			if err != nil {
				t.Fatalf("Expected no error, got %v", err)
			}
			if value != "test-value" {
				t.Errorf("Expected value 'test-value', got '%s'", value)
			}
		})

		t.Run("Error", func(t *testing.T) {
			mockServer.GetSecretFunc = func(ctx context.Context, in *pb.GetSecretRequest) (*pb.Secret, error) {
				return nil, fmt.Errorf("server error")
			}

			_, err := client.GetSecret(context.Background(), "any-ns", "any-id")
			if err == nil {
				t.Fatal("Expected an error, got nil")
			}
			expectedErr := "rpc error: code = Unknown desc = server error"
			if err.Error() != expectedErr {
				t.Errorf("Expected error '%s', got '%v'", expectedErr, err)
			}
		})
	})

	t.Run("LoadEnv", func(t *testing.T) {
		mockServer.ListSecretsFunc = func(ctx context.Context, in *pb.ClientListSecretsRequest) (*pb.ListSecretsResponse, error) {
			return &pb.ListSecretsResponse{
				Namespaces: []*pb.Namespace{
					{Name: "ns-one", Secrets: []*pb.Secret{{Id: "key-one", Value: "val1"}}},
				},
			}, nil
		}

		err := client.LoadEnv(context.Background())
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		val := os.Getenv("GAIA_NS_ONE_KEY_ONE")
		if val != "val1" {
			t.Errorf("Expected env var GAIA_NS_ONE_KEY_ONE to be 'val1', got '%s'", val)
		}
		os.Unsetenv("GAIA_NS_ONE_KEY_ONE") // Clean up
	})
}
