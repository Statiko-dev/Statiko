/*
Copyright © 2020 Alessandro Segala (@ItalyPaleAle)

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published
by the Free Software Foundation, version 3 of the License.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <https://www.gnu.org/licenses/>.
*/

package nodemanager

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"

	"google.golang.org/grpc"

	pb "github.com/statiko-dev/statiko/shared/proto"
)

// RPCServer is the gRPC server that is used to communicate with nodes
type RPCServer struct {
}

// Start the gRPC server
func (s *RPCServer) Start() {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", 2300))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	grpcServer := grpc.NewServer()
	pb.RegisterControllerServer(grpcServer, &controllerServer{})
	grpcServer.Serve(lis)
}

// Implementation for the gRPC Controller server
type controllerServer struct {
}

// GetState is a simple RPC that returns the current state object
func (g *controllerServer) GetState(ctx context.Context, req *pb.GetStateRequest) (*pb.State, error) {
	return &pb.State{}, nil
}

// HealthUpdate is a bi-directional stream that is used by the server to request the health of a node
func (g *controllerServer) HealthUpdate(stream pb.Controller_HealthUpdateServer) error {
	for {
		in, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		fmt.Println(in)
	}
}

// WatchState is a stream that notifies clients of state updates
func (g *controllerServer) WatchState(req *pb.WatchStateRequest, stream pb.Controller_WatchStateServer) error {
	fmt.Println(req)
	stream.Send(&pb.State{
		Secrets: map[string][]byte{"hello": []byte("world")},
	})
	return nil
}
