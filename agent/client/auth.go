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

package client

import (
	"context"
)

// rpcAuth is the object implementing credentials.PerRPCCredentials that provides the auth info
type rpcAuth struct {
	Token string
}

// GetRequestMetadata returns the metadata containing the authorization key
func (a *rpcAuth) GetRequestMetadata(ctx context.Context, in ...string) (map[string]string, error) {
	return map[string]string{
		"authorization": "Bearer " + a.Token,
	}, nil
}

// RequireTransportSecurity returns true because this kind of auth requires TLS
func (a *rpcAuth) RequireTransportSecurity() bool {
	return true
}
