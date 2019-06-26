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
const validator = require('validator')

const utils = require('../shared/utils')
const sitesData = require('../shared/sites-data')
const shared = require('../shared/shared-tests')

// Check that the platform has been started correctly
describe('Sites', function() {
    it('Create site 1', async function() {
        // This operation can take some time
        this.timeout(30 * 1000)
        this.slow(15 * 1000)

        const response = await shared.nodeRequest
            .post('/site')
            .set('Authorization', shared.auth)
            .send(sitesData.site1)
            .expect('Content-Type', /json/)
            .expect(200)

        assert(response.body)
        assert.deepStrictEqual(Object.keys(response.body).sort(), ['id', 'createdAt', 'updatedAt', 'clientCaching', 'tlsCertificate', 'domain', 'aliases'].sort()) 
        assert(validator.isUUID(response.body.id))
        assert(validator.isISO8601(response.body.createdAt, {strict: true}))
        assert(validator.isISO8601(response.body.updatedAt, {strict: true}))
        assert.strictEqual(response.body.clientCaching, sitesData.site1.clientCaching)
        assert.strictEqual(response.body.tlsCertificate, sitesData.site1.tlsCertificate)
        assert.strictEqual(response.body.domain, sitesData.site1.domain)
        assert.deepStrictEqual(response.body.aliases.sort(), sitesData.site1.aliases.sort())

        // Store site
        shared.siteIds.site1 = response.body.id
        shared.sites[response.body.id] = response.body

        // Wait a few moments for the server to finish restarting
        await utils.waitPromise(1500)

        // Check the data directory
        await shared.checkDataDirectory(shared.sites)

        // Check the Nginx configuration
        await shared.checkNginxConfig(shared.sites)

        // Check the cache
        await shared.checkCacheDirectory(shared.sites)
    })

    it('Nginx is up', shared.tests.checkNginxStatus())

    it('Site 1 is up', shared.tests.checkNginxSite(sitesData.site1))

    it('Create site 2', async function() {
        // This operation can take some time
        this.timeout(30 * 1000)
        this.slow(15 * 1000)

        const response = await shared.nodeRequest
            .post('/site')
            .set('Authorization', shared.auth)
            .send(sitesData.site2)
            .expect('Content-Type', /json/)
            .expect(200)

        assert(response.body)
        assert.deepStrictEqual(Object.keys(response.body).sort(), ['id', 'createdAt', 'updatedAt', 'clientCaching', 'tlsCertificate', 'domain', 'aliases'].sort()) 
        assert(validator.isUUID(response.body.id))
        assert(validator.isISO8601(response.body.createdAt, {strict: true}))
        assert(validator.isISO8601(response.body.updatedAt, {strict: true}))
        assert.strictEqual(response.body.clientCaching, sitesData.site2.clientCaching)
        assert.strictEqual(response.body.tlsCertificate, sitesData.site2.tlsCertificate)
        assert.strictEqual(response.body.domain, sitesData.site2.domain)
        assert.deepStrictEqual(response.body.aliases.sort(), sitesData.site2.aliases.sort())

        // Store site
        shared.siteIds.site2 = response.body.id
        shared.sites[response.body.id] = response.body

        // Wait a few moments for the server to finish restarting
        await utils.waitPromise(1500)

        // Check the data directory
        await shared.checkDataDirectory(shared.sites)

        // Check the Nginx configuration
        await shared.checkNginxConfig(shared.sites)

        // Check the cache
        await shared.checkCacheDirectory(shared.sites)
    })

    it('Nginx is up', shared.tests.checkNginxStatus())

    it('Site 2 is up', shared.tests.checkNginxSite(sitesData.site2))

    it('Create site 3', async function() {
        // This operation can take some time
        this.timeout(30 * 1000)
        this.slow(15 * 1000)

        const response = await shared.nodeRequest
            .post('/site')
            .set('Authorization', shared.auth)
            .send(sitesData.site3)
            .expect('Content-Type', /json/)
            .expect(200)

        assert(response.body)
        assert.deepStrictEqual(Object.keys(response.body).sort(), ['id', 'createdAt', 'updatedAt', 'clientCaching', 'tlsCertificate', 'domain', 'aliases'].sort()) 
        assert(validator.isUUID(response.body.id))
        assert(validator.isISO8601(response.body.createdAt, {strict: true}))
        assert(validator.isISO8601(response.body.updatedAt, {strict: true}))
        assert.strictEqual(response.body.clientCaching, sitesData.site3.clientCaching)
        assert.strictEqual(response.body.tlsCertificate, sitesData.site3.tlsCertificate)
        assert.strictEqual(response.body.domain, sitesData.site3.domain)
        assert.deepStrictEqual(response.body.aliases.sort(), sitesData.site3.aliases.sort())

        // Store site
        shared.siteIds.site3 = response.body.id
        shared.sites[response.body.id] = response.body

        // Wait a few moments for the server to finish restarting
        await utils.waitPromise(1500)

        // Check the data directory
        await shared.checkDataDirectory(shared.sites)

        // Check the Nginx configuration
        await shared.checkNginxConfig(shared.sites)

        // Check the cache
        await shared.checkCacheDirectory(shared.sites)
    })

    it('Nginx is up', shared.tests.checkNginxStatus())

    it('Site 3 is up', shared.tests.checkNginxSite(sitesData.site3))

    it('Domain or alias already exists', async function() {
        await shared.nodeRequest
            .post('/site')
            .set('Authorization', shared.auth)
            .send(sitesData.exists1)
            .expect(409)
        await shared.nodeRequest
            .post('/site')
            .set('Authorization', shared.auth)
            .send(sitesData.exists2)
            .expect(409)
    })

    it('List sites', async function() {
        const response = await shared.nodeRequest
            .get('/site')
            .set('Authorization', shared.auth)
            .expect('Content-Type', /json/)
            .expect(200)

        assert(response.body)
        assert(response.body.length == 3)
        for (let i = 0; i < response.body.length; i++) {
            const el = response.body[i]
            assert(el.id)

            const site = shared.sites[el.id]
            assert(site)
            assert.deepStrictEqual(Object.keys(el).sort(), ['id', 'createdAt', 'updatedAt', 'clientCaching', 'tlsCertificate', 'domain', 'aliases'].sort()) 
            assert(validator.isUUID(el.id))
            assert(validator.isISO8601(el.createdAt, {strict: true}))
            assert(validator.isISO8601(el.updatedAt, {strict: true}))
            assert.strictEqual(el.clientCaching, site.clientCaching)
            assert.strictEqual(el.tlsCertificate, site.tlsCertificate)
            assert.strictEqual(el.domain, site.domain)
            assert.deepStrictEqual(el.aliases.sort(), site.aliases.sort())
        }
    })

    it('Status', shared.tests.checkStatus(shared.sites, shared.apps))
})
