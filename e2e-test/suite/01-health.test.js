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

// Check that the platform has been started correctly
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
        assert(Object.keys(response.body).length == 3)
        assert(response.body.authMethod == 'sharedkey')
        assert(response.body.version)
        assert(/(([0-9]{8}\.[0-9]+)|([0-9]+)) (\([0-9a-f]{7}; [0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}\))/.test(response.body.version))
        assert(response.body.hostname)
    })

    it('Get node info via proxy', async function() {
        const response = await shared.nginxRequest
            .get('/info')
            .expect('Content-Type', /json/)
            .expect(200)

        assert(response.body)
        assert(Object.keys(response.body).length == 3)
        assert(response.body.authMethod == 'sharedkey')
        assert(response.body.version) // TODO: Need to validate version
        assert(response.body.hostname)
    })

    it('Check platform data directory', shared.tests.checkDataDirectory())

    it('Check platform config directory', shared.tests.checkConfigDirectory())

    it('Check nginx configuration', shared.tests.checkNginxConfig())
})
