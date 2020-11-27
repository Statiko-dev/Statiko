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

package rpcserver

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"reflect"
	"strings"
	"sync"

	"github.com/spf13/viper"

	"github.com/statiko-dev/statiko/buildinfo"
	controllerutils "github.com/statiko-dev/statiko/controller/utils"
	pb "github.com/statiko-dev/statiko/shared/proto"
	"github.com/statiko-dev/statiko/shared/utils"
)

// GetState is a simple RPC that returns the current state object
func (s *RPCServer) GetState(ctx context.Context, req *pb.GetStateRequest) (*pb.StateMessage, error) {
	// Get the state message
	state, err := s.State.DumpState()
	if err != nil {
		return nil, err
	}
	return state.StateMessage(), nil
}

// Channel is a bi-directional stream that is used for:
// 1. Registering a node
// 2. Allowing the server to request the health of a node
// 3. Notify nodes of state updates
func (s *RPCServer) Channel(stream pb.Controller_ChannelServer) error {
	s.logger.Println("Client connected")
	defer s.logger.Println("Client disconnected")

	// Channel used for responding to health pings
	ch := make(chan chan *pb.NodeHealth)

	// Wait for the node to register itself
	nodeName, err := s.channelRegisterNode(stream, ch)
	if err != nil {
		return err
	}
	// Unregister the node when the channel ends
	defer s.Cluster.UnregisterNode(nodeName)

	// Subscribe to the state
	stateCh := make(chan int)
	s.State.Subscribe(stateCh)
	defer func() {
		s.State.Unsubscribe(stateCh)
		close(stateCh)
	}()

	// Collect the responses when requesting nodes' health
	responseChs := make([]chan *pb.NodeHealth, 0)
	semaphore := sync.Mutex{}

	// Channel to receive messages
	msgCh := clientStreamToChan(stream)

	// Send and receive messages
forloop:
	for {
		select {
		// New message
		case in := <-msgCh:
			if in.Error != nil {
				// Abort
				s.logger.Println("Error while reading message:", in.Error)
				return in.Error
			}
			if in.Done {
				// Exit without error
				s.logger.Println("Stream reached EOF")
				return nil
			}
			if in.Message == nil {
				// Ignore empty messages
				continue forloop
			}

			// Check the type of message
			switch in.Message.Type {
			// New health message received
			case pb.ChannelClientStream_HEALTH_MESSAGE:
				// Store the state version the node is on
				s.Cluster.ReceivedVersion(nodeName, in.Message.Health.Version)

				// If there's no response channel, stop processing here
				if responseChs == nil {
					continue forloop
				}

				// Try sending the response to each channel if they're not closed
				semaphore.Lock()
				for i := 0; i < len(responseChs); i++ {
					if responseChs == nil {
						continue
					}
					select {
					case responseChs[i] <- in.Message.Health:
					default:
					}
				}
				responseChs = make([]chan *pb.NodeHealth, 0)
				semaphore.Unlock()

			// Invalid message
			default:
				s.logger.Printf("node %s sent a message with an invalid type: %d", nodeName, in.Message.Type)
				continue forloop
			}

		// Need to send a ping to request the health
		// Note that this is triggered only after the registration is complete
		case rch := <-ch:
			semaphore.Lock()
			responseChs = append(responseChs, rch)
			semaphore.Unlock()
			err := stream.Send(&pb.ChannelServerStream{
				Type: pb.ChannelServerStream_HEALTH_PING,
			})
			if err != nil {
				s.logger.Println("Error while sending health request:", err)
				return err
			}

		// Send the new state to the clients
		case <-stateCh:
			state, err := s.State.DumpState()
			if err != nil {
				return err
			}
			s.logger.Println("Sending new state to clients", state.Version)
			stream.Send(&pb.ChannelServerStream{
				Type:  pb.ChannelServerStream_STATE_MESSAGE,
				State: state.StateMessage(),
			})

		// Exit if context is done
		case <-stream.Context().Done():
			return nil

		// The server is shutting down
		case <-s.runningCtx.Done():
			return nil
		}
	}
}

// GetTLSCertificate is a simple RPC that returns a TLS certificate
func (s *RPCServer) GetTLSCertificate(ctx context.Context, in *pb.TLSCertificateRequest) (*pb.TLSCertificateMessage, error) {
	// Get the certificate ID
	// Forbid retrieving certificates that belong to the controller or that are from Azure Key Vaut
	certId := in.CertificateId
	if certId == "" || strings.HasPrefix(certId, "akv:") || strings.HasPrefix(certId, "controller_") {
		return nil, errors.New("empty or invalid certificate ID")
	}

	// Get the certificate
	key, cert, err := s.Certs.GetCertificate(certId)
	if err != nil {
		return nil, err
	}
	if len(key) == 0 || len(cert) == 0 {
		return nil, errors.New("certificate not found")
	}

	// Response
	return &pb.TLSCertificateMessage{
		CertificatePem: string(cert),
		KeyPem:         string(key),
	}, nil
}

// GetClusterOptions is a simple RPC that returns the cluster options
func (s *RPCServer) GetClusterOptions(ctx context.Context, in *pb.ClusterOptionsRequest) (msg *pb.ClusterOptions, err error) {
	msg = &pb.ClusterOptions{
		Version:      buildinfo.VersionString(),
		ManifestFile: viper.GetString("manifestFile"),
	}

	// Store
	{
		_, _, obj := controllerutils.GetClusterOptionsStore()
		if obj != nil {
			// Because the interface of msg.Store is unexported, we need to mess a bit with reflection
			// This could panic, but all the types obj could be do implement the interface
			val := reflect.ValueOf(obj)
			reflect.ValueOf(msg).Elem().FieldByName("Store").Set(val)
		}
	}

	// Codesign options
	{
		msg.Codesign = &pb.ClusterOptions_Codesign{
			RequireCodesign: viper.GetBool("codesign.required"),
		}

		// Get the codesign key
		key := s.State.GetCodesignKey()

		// If we don't have a key
		if key == nil || key.E == 0 || key.N == nil {
			msg.Codesign.Type = pb.ClusterOptions_Codesign_NULL
			return msg, nil
		}

		// If we have a key, ensure the exponent is within the bounds we support (uint32)
		if key.E < 1 || key.E > (math.MaxInt32-1) {
			return nil, errors.New("codesign key's exponent is outside of bounds")
		}

		// Create the response message with the RSA key
		msg.Codesign.Type = pb.ClusterOptions_Codesign_RSA
		msg.Codesign.RsaKey = &pb.ClusterOptions_Codesign_RSAKey{
			N: key.N.Bytes(),
			E: uint32(key.E),
		}
	}

	// Notifications
	{
		obj, err := controllerutils.GetClusterOptionsNotifications()
		if err != nil {
			return nil, err
		}
		msg.Notifications = obj
	}

	// Azure Key Vault
	if vaultName := viper.GetString("azureKeyVault.name"); vaultName != "" {
		auth := controllerutils.GetClusterOptionsAzureSP("azureKeyVault")
		if auth == nil {
			return nil, errors.New("azureKeyVault.auth.[tenantId|clientId|clientSecret] are required when azureKeyVault.name is set")
		}
		msg.AzureKeyVault = &pb.ClusterOptions_AzureKeyVault{
			VaultName: vaultName,
			Auth:      auth,
		}
	}

	return msg, nil
}

// GetACMEChallengeResponse is a simple RPC that returns the response to an ACME challenge
func (s *RPCServer) GetACMEChallengeResponse(ctx context.Context, in *pb.ACMEChallengeRequest) (*pb.ACMEChallengeResponse, error) {
	// Ensure the token and domain are set
	if in.Token == "" || in.Domain == "" {
		return nil, errors.New("parameter `token` and `domain` are required")
	}

	// Get the response from the secret store
	message, err := s.State.GetSecret("acme/challenges/" + in.Token)
	if err != nil {
		return nil, err
	}
	parts := strings.SplitN(string(message), "|", 2)

	// Get the site that matches the host header
	site := s.State.GetSite(in.Domain)
	if site == nil {
		return nil, fmt.Errorf("request contained a Host header for a domain or alias that does not exist: %s", in.Domain)
	}

	// Check the host
	if site.Domain != parts[0] && !utils.StringInSlice(site.Aliases, parts[0]) {
		return nil, fmt.Errorf("requested token was for a different host: %s (requested: %s)", parts[0], in.Domain)
	}

	return &pb.ACMEChallengeResponse{
		Response: parts[1],
	}, nil
}

// GetFile is the RPC that returns a file from storage, streaming it chunk-by-chunk
func (s *RPCServer) GetFile(req *pb.FileRequest, stream pb.Controller_GetFileServer) error {
	if req.Name == "" {
		return errors.New("parameter `name` is required")
	}

	// Request the file
	found, data, metadata, err := s.Fs.Get(stream.Context(), req.Name)
	if err != nil {
		return err
	}

	// If the file wasn't found, send an empty message
	if !found || data == nil {
		return stream.Send(&pb.FileStream{})
	}

	// To start, send the metadata as header
	header := make(map[string][]string, len(metadata))
	for k, v := range metadata {
		header[k] = []string{v}
	}
	err = stream.SendHeader(header)
	if err != nil {
		return err
	}

	// Send the data in chunks of 2KB each
	buf := make([]byte, 2<<10)
	var n int
	for {
		// Read a chunk
		n, err = data.Read(buf)
		if err != nil && err != io.EOF {
			return err
		}

		// Send the chunk
		if n > 0 {
			stream.Send(&pb.FileStream{
				// Get up to n bytes as we might have read less than the full buffer
				Chunk: buf[:n],
			})
		}

		// Stop when the stream is over
		if err == io.EOF {
			break
		}
	}

	return nil
}
