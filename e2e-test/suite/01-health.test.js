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

/* eslint eslint-env mocha  */

'use strict'

const assert = require('assert')
const request = require('supertest')
const fs = require('fs')
const promisify = require('util').promisify

const utils = require('../lib/utils')

// Promisified methods
const fsReaddir = promisify(fs.readdir)
const fsReadFile = promisify(fs.readFile)

// Debug
process.env.NODE_TLS_REJECT_UNAUTHORIZED = '0'

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
        
        assert.ok(response.body)
        assert(Object.keys(response.body).length == 3)
        assert(response.body.authMethod == 'sharedkey')
        assert.ok(response.body.version) // TODO: Need to validate version
        assert.ok(response.body.hostname)
    })

    it('Get node info via proxy', async function() {
        const response = await nginxRequest
            .get('/info')
            .expect('Content-Type', /json/)
            .expect(200)
        
        assert.ok(response.body)
        assert(Object.keys(response.body).length == 3)
        assert(response.body.authMethod == 'sharedkey')
        assert.ok(response.body.version) // TODO: Need to validate version
        assert.ok(response.body.hostname)
    })

    it('Check platform root directory', async function() {
        // Test basic filesystem
        assert.ok(await utils.folderExists('/data'))
        assert.ok(await utils.folderExists('/data/apps'))
        assert.ok(await utils.folderExists('/data/cache'))
        assert.ok(await utils.folderExists('/data/sites'))
        assert.deepEqual(await fsReaddir('/data'), ['apps', 'cache', 'sites'])

        // Default app and site
        assert.ok(await utils.folderExists('/data/apps/_default'))
        assert.deepEqual(await fsReaddir('/data/apps'), ['_default'])
        assert((await fsReaddir('/data/apps/_default')).length == 0)
        assert.ok(await utils.folderExists('/data/sites/_default'))
        assert.deepEqual(await fsReaddir('/data/sites'), ['_default'])
        assert.ok(await utils.folderExists('/data/sites/_default/www'))
        assert.ok(await utils.fileExists('/data/sites/_default/nginx-error.log'))
        assert.deepEqual(await fsReaddir('/data/sites/_default'), ['nginx-error.log', 'www'])
        assert((await fsReaddir('/data/sites/_default/www')).length == 0)
    })

    it('Check platform config directory', async function() {
        // Check if directory exists
        assert.ok(await utils.folderExists('/etc/smplatform'))

        // Ensure that the app created the database
        assert.ok(await utils.fileExists('/etc/smplatform/data.db'))
    })

    it('Check nginx configuration', async function() {
        // Check if filesystem is in order
        assert.ok(await utils.folderExists('/etc/nginx'))
        assert.ok(await utils.folderExists('/etc/nginx/conf.d'))
        assert.ok(await utils.fileExists('/etc/nginx/mime.types'))
        assert.ok(await utils.fileExists('/etc/nginx/nginx.conf'))
        assert.deepEqual(await fsReaddir('/etc/nginx'), ['conf.d', 'mime.types', 'nginx.conf'])
        assert.ok(await utils.fileExists('/etc/nginx/conf.d/_default.conf'))
        assert.deepEqual(await fsReaddir('/etc/nginx/conf.d'), ['_default.conf'])

        // Check if the configuration for the default site is correct
        assert.equal(
            (await fsReadFile('/etc/nginx/conf.d/_default.conf', 'utf8')).trim(),
            (await fsReadFile('fixtures/nginx-default-site.conf', 'utf8')).trim()
        )
    })
})
