/*
Copyright © 2019 Alessandro Segala (@ItalyPaleAle)

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

/* eslint-env mocha  */

'use strict'

const assert = require('assert')

const shared = require('../shared/shared-tests')

describe('Health check', function() {
    it('Node is up', function() {
        return shared.nodeRequest
            .get('/')
            .expect(404) // This should correctly return 404
    })

    it('Nginx is up', shared.tests.checkNginxStatus())

    it('Get node info from /info', async function() {
        const response = await shared.nodeRequest
            .get('/info')
            .expect('Content-Type', /json/)
            .expect(200)

        assert(response.body)
        assert(Object.keys(response.body).length == 4)
        assert.deepEqual(response.body.authMethods.sort(), ['azureAD', 'psk'])
        assert(response.body.version)
        // If we're in a dev environment, version might be empty (" (; )")
        if (response.body.version == ' (; )') {
            // eslint-disable-next-line no-console
            console.warn('WARN: version is empty - are we in a dev environment?')
        }
        else {
            assert(/(v[0-9\.]+(-[a-z0-9\.]+)?|canary) (\([0-9a-f]{7}; [0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z?\))/.test(response.body.version), 'Invalid value for version: ' + response.body.version)
        }
        assert(response.body.hostname)
        assert(response.body.azureAD)
        assert.deepEqual(Object.keys(response.body.azureAD).sort(), ['authorizeUrl', 'clientId', 'tokenUrl'])
        assert(response.body.azureAD.authorizeUrl.startsWith('https://login.microsoftonline.com'))
        assert(response.body.azureAD.tokenUrl.startsWith('https://login.microsoftonline.com'))
        assert.equal(response.body.azureAD.clientId.length, 36)
    })

    it('Get node info via proxy', async function() {
        const response = await shared.nginxRequest
            .get('/info')
            .expect('Content-Type', /json/)
            .expect(200)

        assert(response.body)
        assert(Object.keys(response.body).length == 4)
        assert.deepEqual(response.body.authMethods.sort(), ['azureAD', 'psk'])
        assert(response.body.version)
        // If we're in a dev environment, version might be empty (" (; )")
        if (response.body.version == ' (; )') {
            // eslint-disable-next-line no-console
            console.warn('WARN: version is empty - are we in a dev environment?')
        }
        else {
            assert(/(v[0-9\.]+(-[a-z0-9\.]+)?|canary) (\([0-9a-f]{7}; [0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z?\))/.test(response.body.version), 'Invalid value for version: ' + response.body.version)
        }
        assert(response.body.hostname)
        assert(response.body.azureAD)
        assert.deepEqual(Object.keys(response.body.azureAD).sort(), ['authorizeUrl', 'clientId', 'tokenUrl'])
        assert(response.body.azureAD.authorizeUrl.startsWith('https://login.microsoftonline.com'))
        assert(response.body.azureAD.tokenUrl.startsWith('https://login.microsoftonline.com'))
        assert.equal(response.body.azureAD.clientId.length, 36)
    })

    it('Check platform data directory', shared.tests.checkDataDirectory())

    it('Check platform config directory', shared.tests.checkConfigDirectory())

    it('Check nginx configuration', shared.tests.checkNginxConfig())
})
