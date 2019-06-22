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
const validator = require('validator')

const sitesData = require('../shared/sites-data')
const sharedTests = require('../shared/shared-tests')

// Read URLs from env vars
const nodeUrl = process.env.NODE_URL || 'https://localhost:3000'
const nginxUrl = process.env.NGINX_URL || 'https://localhost'

// Auth header
const auth = 'hello world'

// Supertest instances
const nodeRequest = request(nodeUrl)
const nginxRequest = request(nginxUrl)

// Site ids
const siteIds = {}

// Configured sites and apps
const sites = {}

// Check that the platform has been started correctly
describe('SMPlatform node', function() {
    it('Adopt node', async function() {
        // This operation can take some time
        this.timeout(10 * 1000)
        this.slow(5 * 1000)

        const response = await nodeRequest
            .post('/adopt')
            .set('Authorization', auth)
            .expect('Content-Type', /json/)
            .expect(200)
        
        assert(response.body)
        assert.deepStrictEqual(response.body, {message: 'Adopted'})
    })

    it('Check platform data directory', sharedTests.tests.checkDataDirectory(sites))

    it('Check platform config directory', sharedTests.tests.checkConfigDirectory())

    it('Check nginx configuration', sharedTests.tests.checkNginxConfig())

    it('Create site 1', async function() {
        // This operation can take some time
        this.timeout(30 * 1000)
        this.slow(15 * 1000)

        const response = await nodeRequest
            .post('/site')
            .set('Authorization', auth)
            .send(sitesData.site1)
            .expect('Content-Type', /json/)
            .expect(200)
        
        assert(response.body)
        assert.deepStrictEqual(Object.keys(response.body).sort(), ['ID', 'createdAt', 'updatedAt', 'clientCaching', 'tlsCertificate', 'domain', 'aliases'].sort()) 
        assert(validator.isUUID(response.body.ID))
        assert(validator.isISO8601(response.body.createdAt, {strict: true}))
        assert(validator.isISO8601(response.body.updatedAt, {strict: true}))
        assert.strictEqual(response.body.clientCaching, true)
        assert.strictEqual(response.body.tlsCertificate, sitesData.site1.tlsCertificate)
        assert.strictEqual(response.body.domain, sitesData.site1.domain)
        assert.deepStrictEqual(response.body.aliases.sort(), sitesData.site1.aliases.sort())

        // Store site
        siteIds.site1 = response.body.ID
        sites[response.body.ID] = response.body

        // Check the data directory
        await sharedTests.checkDataDirectory(sites)

        // Check the Nginx configuration
        await sharedTests.checkNginxConfig(sites)
    })

    it('Nginx is up', function() {
        return nginxRequest
            .get('/')
            .expect(403) // This should fail with a 403
    })
})
