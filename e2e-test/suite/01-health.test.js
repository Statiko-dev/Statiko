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
const request = require('supertest')

const sharedTests = require('../shared/shared-tests')

// Read URLs from env vars
const nodeUrl = process.env.NODE_URL || 'https://localhost:3000'
const nginxUrl = process.env.NGINX_URL || 'https://localhost'

// Supertest instances
const nodeRequest = request(nodeUrl)
const nginxRequest = request(nginxUrl)

// Check that the platform has been started correctly
describe('SMPlatform health', function() {
    it('Node is up', function() {
        return nodeRequest
            .get('/')
            .expect(404) // This should correctly return 404
    })

    it('Nginx is up', function() {
        return nginxRequest
            .get('/')
            .expect(403) // This should fail with a 403
    })

    it('Get node info from /info', async function() {
        const response = await nodeRequest
            .get('/info')
            .expect('Content-Type', /json/)
            .expect(200)
        
        assert(response.body)
        assert(Object.keys(response.body).length == 3)
        assert(response.body.authMethod == 'sharedkey')
        assert(response.body.version)
        assert(/[0-9]{8}\.[0-9]+ \([0-9a-f]{7}; [0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}\)/.test(response.body.version))
        assert(response.body.hostname)
    })

    it('Get node info via proxy', async function() {
        const response = await nginxRequest
            .get('/info')
            .expect('Content-Type', /json/)
            .expect(200)
        
        assert(response.body)
        assert(Object.keys(response.body).length == 3)
        assert(response.body.authMethod == 'sharedkey')
        assert(response.body.version) // TODO: Need to validate version
        assert(response.body.hostname)
    })

    it('Check platform data directory', sharedTests.tests.checkDataDirectory())

    it('Check platform config directory', sharedTests.tests.checkConfigDirectory())

    it('Check nginx configuration', sharedTests.tests.checkNginxConfig())
})
