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

package app

import (
	"crypto/tls"

	"github.com/spf13/viper"
	"github.com/statiko-dev/statiko/controller/certificates"
	"github.com/statiko-dev/statiko/controller/state"
	"github.com/statiko-dev/statiko/shared/certutils"
	pb "github.com/statiko-dev/statiko/shared/proto"
	"github.com/statiko-dev/statiko/shared/utils"
)

// GetControllerCertificate returns the certificate for the controller node
// The order is of priority is:
// 1. Certificate loaded from disk if "controller.tls.certificate" and "controller.tls.key" are set and the files exist
// 2. Azure Key Vault if option "controller.tls.azureKeyVault" is set
// 3. ACME if "controller.tls.acme" is enabled
// 4. Self-signed certificate otherwise
func (c *Controller) GetControllerCertificate() (cert *tls.Certificate, err error) {
	// First, try loading from disk
	cert, err = c.certificateFromDisk()
	if err != nil || cert != nil {
		return
	}

	// Try loading from Azure Key Vault
	cert, err = c.certificateFromAzureKeyVault()
	if err != nil || cert != nil {
		return
	}

	// Whether we are using ACME or a self-signed certificate, retrieve the certificate from the state
	// If there's no certificate, this will generate a self-signed one
	cert, err = c.certificateFromState()
	return
}

// Try loading a certificate from disk
func (c *Controller) certificateFromDisk() (cert *tls.Certificate, err error) {
	var exist bool

	// Check if "controller.tls.certificate" and "controller.tls.key" are set
	certFile := viper.GetString("controller.tls.certificate")
	keyFile := viper.GetString("controller.tls.key")
	if certFile == "" || keyFile == "" {
		// No error because not found
		return nil, nil
	}

	// Ensure the files exist, as there might just be the default value
	exist, err = utils.FileExists(certFile)
	if err != nil {
		return nil, err
	}
	if !exist {
		c.logger.Printf("[Warn] `controller.tls.certificate` is set to `%s`, but the file does not exist - skipping loading certificate from disk\n", certFile)
		// No error because it might just be using the default option
		return nil, nil
	}
	exist, err = utils.FileExists(keyFile)
	if err != nil {
		return nil, err
	}
	if !exist {
		c.logger.Printf("[Warn] `controller.tls.key` is set to `%s`, but the file does not exist - skipping loading certificate from disk\n", keyFile)
		// No error because it might just be using the default option
		return nil, nil
	}

	// Load the files
	obj, err := tls.LoadX509KeyPair(certFile, keyFile)
	return &obj, err
}

// Try loading a certificate from disk
func (c *Controller) certificateFromAzureKeyVault() (cert *tls.Certificate, err error) {
	// Check if we have the option
	name := viper.GetString("controller.tls.azureKeyVault")
	if name == "" {
		// No error because not found
		return nil, nil
	}

	// Request the certificate
	keyPem, certPem, err := certutils.GetAKVCertificate(name, c.AKV)
	if err != nil {
		return nil, err
	}

	// Return the tls.Certificate object
	obj, err := tls.X509KeyPair(certPem, keyPem)
	return &obj, err
}

// Loading a certificate from the state store (for both ACME and self-signed)
// If there's no certificate in the store, generate a new self-signed one
func (c *Controller) certificateFromState() (cert *tls.Certificate, err error) {
	// Convention is that controller certs start with "controller_"
	nodeName := viper.GetString("nodeName")
	certId := "controller_" + nodeName

	// Retrieve the certificate (if present)
	keyPem, certPem, err := c.State.GetCertificate(certId)
	if err != nil && err != state.ErrCertificateNotFound {
		return nil, err
	}

	// If we have a certificate, return that
	if len(keyPem) > 0 && len(certPem) > 0 {
		obj, err := tls.X509KeyPair(certPem, keyPem)
		return &obj, err
	}

	// Generate a new self-signed certificate using the nodeName as domain
	c.logger.Println("Generating a self-signed certificate for the controller node", nodeName)
	keyPem, certPem, err = certificates.GenerateTLSCert(nodeName)
	if err != nil {
		return nil, err
	}

	// Get the x509 object to set the other properties
	certX509, err := certificates.GetX509(certPem)
	if err != nil {
		return nil, err
	}

	// Save the certificate
	certObj := &pb.TLSCertificate{
		Type: pb.TLSCertificate_SELF_SIGNED,
	}
	certObj.SetCertificateProperties(certX509)
	err = c.State.SetCertificate(certObj, certId, keyPem, certPem)
	if err != nil {
		return nil, err
	}

	// Return the tls.Certificate object
	obj, err := tls.X509KeyPair(certPem, keyPem)
	return &obj, err
}
