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
const fs = require('fs')
const {promisify} = require('util')

const fsReadFile = promisify(fs.readFile)

const {cloneObject} = require('../shared/utils')
const sitesData = require('../shared/sites-data')
const appData = require('../shared/app-data')
const tlsData = require('../shared/tls-data')
const shared = require('../shared/shared-tests')

describe('Manage sites', function() {
    // Sites deployed
    // In the previous test, we had these 2 deployed
    const deployed = [
        cloneObject(sitesData.site2app2),
        cloneObject(sitesData.site3app3)
    ]

    // Apps deployed
    const apps = []

    // Function that returns the object for a given site
    const findSite = (domain) => {
        for (const k in deployed) {
            if (deployed.hasOwnProperty(k)) {
                if (deployed[k].domain == domain) {
                    return deployed[k]
                }
            }
        }
        return null
    }

    it('Add site1', async function() {
        // Request
        const deploy = cloneObject(sitesData.site1)
        const response = await shared.nodeRequest
            .post('/site')
            .set('Authorization', shared.auth)
            .send(deploy)
            .expect(204)

        // Tests
        assert(response.text.length == 0)

        // Add to list of sites deployed
        deployed.unshift(deploy)
    })

    it('Wait for sync', shared.tests.waitForSync())

    it('Check data directory', shared.tests.checkDataDirectory(deployed))

    it('Check nginx configuration', shared.tests.checkNginxConfig(deployed))

    it('Test site1 health', shared.tests.checkNginxSite(sitesData.site1))

    it('Check status', shared.tests.checkStatus(deployed))

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
        // Request
        const response = await shared.nodeRequest
            .get('/site')
            .set('Authorization', shared.auth)
            .expect('Content-Type', /json/)
            .expect(200)

        // Tests
        assert(response.body)
        assert(Array.isArray(response.body))
        assert(response.body.length == 3)

        for (let i = 0; i < response.body.length; i++) {
            const el = response.body[i]
            assert(el.domain)

            // Look for the corresponding site object
            const site = findSite(el.domain)
            assert(site)

            assert.deepStrictEqual(Object.keys(el).sort(), ['tlsCertificate', 'tlsCertificateVersion', 'domain', 'aliases', 'error', 'app'].sort())
            assert(!el.error)
            assert.strictEqual(el.tlsCertificate, site.tlsCertificate)
            if (site.tlsCertificate) {
                assert.strictEqual(el.tlsCertificateVersion, tlsData[el.tlsCertificate])
            }
            assert.deepStrictEqual(el.aliases.sort(), site.aliases.sort())

            // Is there an app deployed?
            if (site.app && site.app.name) {
                assert.deepStrictEqual(el.app, site.app)
            }
            else {
                assert.strictEqual(el.app, null)
            }
        }
    })

    it('Get site details', async function() {
        let site, response = null

        // Request site1
        response = await shared.nodeRequest
            .get('/site/site1.local')
            .set('Authorization', shared.auth)
            .expect('Content-Type', /json/)
            .expect(200)

        // Tests for site1
        site = findSite('site1.local')
        assert(response.body)
        assert.deepStrictEqual(Object.keys(response.body).sort(), ['tlsCertificate', 'tlsCertificateVersion', 'domain', 'aliases', 'error', 'app'].sort())
        assert(!response.body.error)
        assert.strictEqual(response.body.tlsCertificate, site.tlsCertificate)
        if (site.tlsCertificate) {
            assert.strictEqual(response.body.tlsCertificateVersion, tlsData[response.body.tlsCertificate])
        }
        assert.deepStrictEqual(response.body.aliases.sort(), site.aliases.sort())
        assert.deepStrictEqual(response.body.app, null)

        // Request site2 using an alias
        response = await shared.nodeRequest
            .get('/site/site2-alias.local')
            .set('Authorization', shared.auth)
            .expect('Content-Type', /json/)
            .expect(200)

        // Tests for site2
        site = findSite('site2.local')
        assert(response.body)
        assert.deepStrictEqual(Object.keys(response.body).sort(), ['tlsCertificate', 'tlsCertificateVersion', 'domain', 'aliases', 'error', 'app'].sort())
        assert(!response.body.error)
        assert.strictEqual(response.body.tlsCertificate, site.tlsCertificate)
        if (site.tlsCertificate) {
            assert.strictEqual(response.body.tlsCertificateVersion, tlsData[response.body.tlsCertificate])
        }
        assert.deepStrictEqual(response.body.aliases.sort(), site.aliases.sort())
        assert.deepStrictEqual(response.body.app, site.app)

        // Test a site that doesn't exist
        await shared.nodeRequest
            .get('/site/doesntexist.com')
            .set('Authorization', shared.auth)
            .expect(404)
    })

    it('Update site1 configuration', async function() {
        // This operation can take some time
        this.timeout(15 * 1000)
        this.slow(8 * 1000)

        // Update site1 multiple times
        const site = cloneObject(findSite('site1.local'))
        for (let i = 1; i <= 3; i++) {
            await shared.nodeRequest
                .patch('/site/site1.local')
                .set('Authorization', shared.auth)
                .send(sitesData['site1patch' + i])
                .expect(204)

            // Update the local data
            Object.assign(site, sitesData['site1patch' + i])

            // Request the site's details
            const response = await shared.nodeRequest
                .get('/site/site1.local')
                .set('Authorization', shared.auth)
                .expect('Content-Type', /json/)
                .expect(200)

            // Tests
            assert(response.body)
            assert.deepStrictEqual(Object.keys(response.body).sort(), ['tlsCertificate', 'tlsCertificateVersion', 'domain', 'aliases', 'error', 'app'].sort())
            assert(!response.body.error)
            assert.strictEqual(response.body.tlsCertificate, site.tlsCertificate)
            if (site.tlsCertificate) {
                assert.strictEqual(response.body.tlsCertificateVersion, tlsData[response.body.tlsCertificate])
            }
            assert.deepStrictEqual(response.body.aliases.sort(), site.aliases.sort())
            assert.deepStrictEqual(response.body.app, null)

            // Check the nginx config
            // Skip this for patch 3 as it shouldn't change
            if (i <= 2) {
                assert.equal(
                    (await fsReadFile('/etc/nginx/conf.d/site1.local.conf', 'utf8')).trim(),
                    (await fsReadFile('fixtures/nginx-site1patch' + i + '.conf', 'utf8')).trim()
                )
            }
        }

        // Update the site in the deployed list
        deployed[0] = site
    })

    it('Wait for sync', shared.tests.waitForSync())

    it('Check status', shared.tests.checkStatus(deployed))

    it('Deploy app to site1', async function() {
        // Request
        const app = {
            name: appData.app1.app,
            version: appData.app1.version,
        }
        const response = await shared.nodeRequest
            .put('/site/site1.local/app')
            .set('Authorization', shared.auth)
            .send(app)
            .expect(204)

        // Tests
        assert(response.text.length == 0)

        // Update the site in the deployed list
        // Then add the app
        deployed[0].app = app
        apps.push(app)
    })

    it('Wait for sync', shared.tests.waitForSync())

    it('Check cache directory', shared.tests.checkCacheDirectory(deployed, apps))

    it('Check data directory', shared.tests.checkDataDirectory(deployed))

    it('Test site1 health', shared.tests.checkNginxSiteIndex(deployed, 0, appData.app1))

    it('Check status', shared.tests.checkStatus(deployed))

    it('Delete site1', async function() {
        // Request
        const response = await shared.nodeRequest
            .delete('/site/site1.local')
            .set('Authorization', shared.auth)
            .expect(204)

        // Tests
        assert(response.text.length == 0)

        // Add to list of sites deployed
        deployed.splice(0, 1)
    })

    it('Wait for sync', shared.tests.waitForSync())

    it('Check data directory', shared.tests.checkDataDirectory(deployed))

    it('Check nginx configuration', shared.tests.checkNginxConfig(deployed))

    it('Check status', shared.tests.checkStatus(deployed))
})
