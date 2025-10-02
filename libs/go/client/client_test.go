package client

import (
	"context"
	"fmt"
	"net"
	"os"
	"reflect"
	"testing"

	pb "github.com/stain-win/gaia/libs/go/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/emptypb"
)

// mockGaiaClientServer is a mock of the GaiaClientServer interface for testing.
type mockGaiaClientServer struct {
	pb.UnimplementedGaiaClientServer // Embed for forward compatibility
	GetSecretFunc                    func(ctx context.Context, in *pb.GetSecretRequest) (*pb.Secret, error)
	GetStatusFunc                    func(ctx context.Context, in *emptypb.Empty) (*pb.StatusResponse, error)
	GetNamespacesFunc                func(ctx context.Context, in *emptypb.Empty) (*pb.NamespaceResponse, error)
	GetCommonSecretsFunc             func(ctx context.Context, in *pb.GetCommonSecretsRequest) (*pb.GetCommonSecretsResponse, error)
}

func (m *mockGaiaClientServer) GetSecret(ctx context.Context, in *pb.GetSecretRequest) (*pb.Secret, error) {
	return m.GetSecretFunc(ctx, in)
}

func (m *mockGaiaClientServer) GetStatus(ctx context.Context, in *emptypb.Empty) (*pb.StatusResponse, error) {
	return m.GetStatusFunc(ctx, in)
}

func (m *mockGaiaClientServer) GetNamespaces(ctx context.Context, in *emptypb.Empty) (*pb.NamespaceResponse, error) {
	return m.GetNamespacesFunc(ctx, in)
}

func (m *mockGaiaClientServer) GetCommonSecrets(ctx context.Context, in *pb.GetCommonSecretsRequest) (*pb.GetCommonSecretsResponse, error) {
	return m.GetCommonSecretsFunc(ctx, in)
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

	conn, err := grpc.DialContext(context.Background(), "",
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
	mockServer := &mockGaiaClientServer{}
	conn, cleanup := startTestServer(mockServer)
	defer cleanup()

	client := &Client{
		conn:   conn,
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

	t.Run("GetCommonSecrets", func(t *testing.T) {
		mockServer.GetCommonSecretsFunc = func(ctx context.Context, in *pb.GetCommonSecretsRequest) (*pb.GetCommonSecretsResponse, error) {
			resp := &pb.GetCommonSecretsResponse{
				Namespaces: []*pb.Namespace{
					{Name: "ns1", Secrets: []*pb.Secret{{Id: "key1", Value: "val1"}}},
					{Name: "ns2", Secrets: []*pb.Secret{{Id: "key2", Value: "val2"}}},
				},
			}
			if in.Namespace != nil && *in.Namespace == "ns1" {
				return &pb.GetCommonSecretsResponse{Namespaces: []*pb.Namespace{resp.Namespaces[0]}}, nil
			}
			return resp, nil
		}

		t.Run("AllNamespaces", func(t *testing.T) {
			secrets, err := client.GetCommonSecrets(context.Background())
			if err != nil {
				t.Fatalf("Expected no error, got %v", err)
			}
			expected := map[string]map[string]string{
				"ns1": {"key1": "val1"},
				"ns2": {"key2": "val2"},
			}
			if !reflect.DeepEqual(secrets, expected) {
				t.Errorf("Expected secrets %v, got %v", expected, secrets)
			}
		})

		t.Run("SpecificNamespace", func(t *testing.T) {
			secrets, err := client.GetCommonSecrets(context.Background(), "ns1")
			if err != nil {
				t.Fatalf("Expected no error, got %v", err)
			}
			expected := map[string]map[string]string{
				"ns1": {"key1": "val1"},
			}
			if !reflect.DeepEqual(secrets, expected) {
				t.Errorf("Expected secrets %v, got %v", expected, secrets)
			}
		})
	})

	t.Run("LoadEnv", func(t *testing.T) {
		mockServer.GetCommonSecretsFunc = func(ctx context.Context, in *pb.GetCommonSecretsRequest) (*pb.GetCommonSecretsResponse, error) {
			return &pb.GetCommonSecretsResponse{
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

	t.Run("GetStatus", func(t *testing.T) {
		mockServer.GetStatusFunc = func(ctx context.Context, in *emptypb.Empty) (*pb.StatusResponse, error) {
			return &pb.StatusResponse{Status: "running"}, nil
		}

		status, err := client.GetStatus(context.Background())
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if status != "running" {
			t.Errorf("Expected status 'running', got '%s'", status)
		}
	})

	t.Run("GetNamespaces", func(t *testing.T) {
		mockServer.GetNamespacesFunc = func(ctx context.Context, in *emptypb.Empty) (*pb.NamespaceResponse, error) {
			return &pb.NamespaceResponse{Namespaces: []string{"ns1", "ns2"}}, nil
		}

		namespaces, err := client.GetNamespaces(context.Background())
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		expected := []string{"ns1", "ns2"}
		if !reflect.DeepEqual(namespaces, expected) {
			t.Errorf("Expected namespaces %v, got %v", expected, namespaces)
		}
	})
}
