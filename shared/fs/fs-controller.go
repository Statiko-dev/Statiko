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

package fs

import (
	"context"
	"errors"
	"io"
	"strings"

	"google.golang.org/grpc"

	pb "github.com/statiko-dev/statiko/shared/proto"
)

// Options for initializing a Controller fs
type ControllerFsOpts struct {
	// gRPC client
	RPC pb.ControllerClient
}

// Controller is used by agents to request files directly from the controller
// This is needed when the type of fs is "local" for the controller, as files are stored in the controller's local disk
// This provider implements only the Get method
type Controller struct {
	RPC pb.ControllerClient
}

func (f *Controller) Init(optsI interface{}) error {
	// Cast opts to ControllerFsOpts
	opts, ok := optsI.(*ControllerFsOpts)
	if !ok || opts == nil || opts.RPC == nil {
		return errors.New("invalid argument")
	}

	// Get the RPC client
	f.RPC = opts.RPC

	return nil
}

func (f *Controller) Get(parentCtx context.Context, name string) (found bool, data io.ReadCloser, metadata map[string]string, err error) {
	if name == "" || strings.HasPrefix(name, ".metadata.") {
		return false, nil, nil, ErrNameEmptyInvalid
	}

	ctx, cancel := context.WithCancel(parentCtx)

	// Request the file from the gRPC server
	req := &pb.FileRequest{
		Name: name,
	}
	var res pb.Controller_GetFileClient
	res, err = f.RPC.GetFile(ctx, req, grpc.WaitForReady(true))
	if err != nil {
		cancel()
		return false, nil, nil, err
	}

	// First, get the metadata
	header, err := res.Header()
	if err != nil {
		cancel()
		return false, nil, nil, err
	}
	if header != nil && len(header) > 0 {
		metadata = make(map[string]string, len(header))
		for k, v := range header {
			// Get the last value only
			metadata[k] = v[len(v)-1]
		}
	}

	// Get the first chunk to check if it's empty
	// If it's an empty message, it means that the file doesn't exist
	read, err := res.Recv()
	if err != nil {
		cancel()
		return false, nil, nil, err
	}
	if read == nil || len(read.Chunk) == 0 {
		// File doesn't exist
		cancel()
		return false, nil, nil, nil
	}

	// Create the pipe for the data
	var w *io.PipeWriter
	data, w = io.Pipe()

	// In a goroutine, send the data to the pipe
	go func(initChunk []byte, res pb.Controller_GetFileClient) {
		defer cancel()

		// Send the data we already have
		_, err := w.Write(initChunk)
		if err != nil {
			// Ignore errors here
			_ = w.CloseWithError(err)
			return
		}

		// Receive all messages until it's done
		var read *pb.FileStream
		for {
			read, err = res.Recv()
			if err == io.EOF {
				// It's done
				// Ignore errors here
				_ = w.Close()
				return
			}
			if err != nil {
				// There's an error (besides EOF)
				// Ignore errors here
				_ = w.CloseWithError(err)
				return
			}

			// Send the chunk
			if len(read.Chunk) > 0 {
				_, err = w.Write(read.Chunk)
				if err != nil {
					// Ignore errors here
					_ = w.CloseWithError(err)
					return
				}
			}
		}
	}(read.Chunk, res)

	return true, data, metadata, nil
}

func (f *Controller) List(ctx context.Context) (info []FileInfo, err error) {
	// Not implemented by this fs
	err = errors.New("this fs does not implement the List method")
	return
}

func (f *Controller) Set(ctx context.Context, name string, in io.Reader, metadata map[string]string) (err error) {
	// Not implemented by this fs
	err = errors.New("this fs does not implement the Set method")
	return
}

func (f *Controller) GetMetadata(ctx context.Context, name string) (metadata map[string]string, err error) {
	// Not implemented by this fs
	err = errors.New("this fs does not implement the GetMetadata method")
	return
}

func (f *Controller) SetMetadata(ctx context.Context, name string, metadata map[string]string) (err error) {
	// Not implemented by this fs
	err = errors.New("this fs does not implement the SetMetadata method")
	return
}

func (f *Controller) Delete(ctx context.Context, name string) (err error) {
	// Not implemented by this fs
	err = errors.New("this fs does not implement the Delete method")
	return
}
