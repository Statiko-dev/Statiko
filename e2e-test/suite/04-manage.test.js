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

const {cloneObject} = require('../shared/utils')
const sitesData = require('../shared/sites-data')
const tlsData = require('../shared/tls-data')
const shared = require('../shared/shared-tests')

describe('Manage sites', function() {
    // Sites deployed
    // In the previous test, we had these 2 deployed
    const deployed = [
        cloneObject(sitesData.site2app2),
        cloneObject(sitesData.site3app3)
    ]

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
        deployed.push(deploy)
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

        for (let i = 0; i < response.body.length; i++) {
            const el = response.body[i]
            assert(el.domain)

            // Look for the corresponding site object
            const site = findSite(el.domain)
            assert(site)

            assert.deepStrictEqual(Object.keys(el).sort(), ['clientCaching', 'tlsCertificate', 'tlsCertificateVersion', 'domain', 'aliases', 'error', 'app'].sort())
            assert(!el.error)
            assert.strictEqual(el.clientCaching, site.clientCaching)
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
})
