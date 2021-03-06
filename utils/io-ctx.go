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

package utils

import (
	"context"
	"io"
)

type ioctx func(p []byte) (int, error)

func (f ioctx) Write(p []byte) (n int, err error) {
	return f(p)
}
func (f ioctx) Read(p []byte) (n int, err error) {
	return f(p)
}

func WriterFuncWithContext(ctx context.Context, in io.Writer) ioctx {
	return func(p []byte) (int, error) {
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		default:
			return in.Write(p)
		}
	}
}

func ReaderFuncWithContext(ctx context.Context, in io.Reader) ioctx {
	return func(p []byte) (int, error) {
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		default:
			return in.Read(p)
		}
	}
}
