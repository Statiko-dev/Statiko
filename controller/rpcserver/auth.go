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
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"

	"github.com/spf13/viper"
	"github.com/statiko-dev/statiko/shared/utils"
)

// AuthGRPCUnaryInterceptor returns an interceptor for unary ("simple") gRPC requests that checks the authorization field in the metadata
// The "excludeMethods" slice contains an optional list of full method names that don't require authentication
func AuthGRPCUnaryInterceptor(excludeMethods []string) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
		// Check if this method is always allowed, even without authorization
		if len(excludeMethods) > 0 && utils.StringInSlice(excludeMethods, info.FullMethod) {
			// Skip checking authorization and just continue the execution
			return handler(ctx, req)
		}

		// Check if the call is authorized
		err = authGRPCCheckMetadata(ctx)
		if err != nil {
			return
		}

		// Call is authorized, so continue the execution
		return handler(ctx, req)
	}
}

// AuthGRPCStreamInterceptor is an interceptor for stream gRPC requests that checks the authorization field in the metadata
func AuthGRPCStreamInterceptor(srv interface{}, srvStream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) (err error) {
	// Check if the call is authorized
	err = authGRPCCheckMetadata(srvStream.Context())
	if err != nil {
		return
	}

	// Call is authorized, so continue the execution
	return handler(srv, srvStream)
}

// Used by the gRPC auth interceptors, this checks the authorization metadata
func authGRPCCheckMetadata(ctx context.Context) error {
	// Ensure we have an authorization metadata
	// Note that the keys in the metadata object are always lowercased
	m, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return grpc.Errorf(codes.Unauthenticated, "missing metadata")
	}
	if len(m["authorization"]) != 1 {
		return grpc.Errorf(codes.Unauthenticated, "invalid authorization")
	}

	// Remove the "Bearer " prefix if present
	token := strings.TrimPrefix(m["authorization"][0], "Bearer ")

	// Validate the token
	if token == "" || token != viper.GetString("controller.token") {
		return grpc.Errorf(codes.Unauthenticated, "invalid authorization")
	}
	return nil
}
