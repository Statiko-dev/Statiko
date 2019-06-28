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

const appData = require('../shared/app-data')
const sitesData = require('../shared/sites-data')
const shared = require('../shared/shared-tests')

// Check that the platform has been started correctly
describe('Apps', function() {
    it('Deploy app 1', async function() {
        // This operation can take some time
        this.timeout(30 * 1000)
        this.slow(15 * 1000)

        const response = await shared.nodeRequest
            .post('/site/' + sitesData.site1.domain + '/deploy')
            .set('Authorization', shared.auth)
            .send(appData.app1)
            .expect('Content-Type', /json/)
            .expect(200)
        
        assert(response)
        assert.deepStrictEqual(Object.keys(response.body).sort(), ['id', 'site', 'app', 'version', 'deploymentError', 'status', 'time'].sort()) 
        assert(validator.isUUID(response.body.id))
        assert(response.body.time === null)
        assert.strictEqual(response.body.site, shared.siteIds.site1)
        assert.strictEqual(response.body.app, appData.app1.app)
        assert.strictEqual(response.body.version, appData.app1.version)
        assert.strictEqual(response.body.deploymentError, null)
        assert.strictEqual(response.body.status, 'running')

        // Wait for app to be deployed
        await shared.waitForDeployment(sitesData.site1.domain, appData.app1)

        // Add deployed app to the list
        shared.apps[sitesData.site1.domain] = appData.app1

        // App's bundle is in cache
        await shared.checkCacheDirectory(shared.sites, shared.apps)
    })

    it('Check platform data directory', shared.tests.checkDataDirectory(shared.sites, shared.apps))

    it('App 1 is up', shared.tests.checkNginxSite(sitesData.site1, appData.app1))

    it('Deploy app 2', async function() {
        // This operation can take some time
        this.timeout(30 * 1000)
        this.slow(15 * 1000)

        const response = await shared.nodeRequest
            .post('/site/' + sitesData.site2.domain + '/deploy')
            .set('Authorization', shared.auth)
            .send(appData.app2)
            .expect('Content-Type', /json/)
            .expect(200)

        assert(response)
        assert.deepStrictEqual(Object.keys(response.body).sort(), ['id', 'site', 'app', 'version', 'deploymentError', 'status', 'time'].sort()) 
        assert(validator.isUUID(response.body.id))
        assert(response.body.time === null)
        assert.strictEqual(response.body.site, shared.siteIds.site2)
        assert.strictEqual(response.body.app, appData.app2.app)
        assert.strictEqual(response.body.version, appData.app2.version)
        assert.strictEqual(response.body.deploymentError, null)
        assert.strictEqual(response.body.status, 'running')

        // Wait for app to be deployed
        await shared.waitForDeployment(sitesData.site2.domain, appData.app2)

        // Add deployed app to the list
        shared.apps[sitesData.site2.domain] = appData.app2

        // App's bundle is in cache
        await shared.checkCacheDirectory(shared.sites, shared.apps)
    })

    it('Check platform data directory', shared.tests.checkDataDirectory(shared.sites, shared.apps))

    it('App 2 is up', shared.tests.checkNginxSite(sitesData.site2, appData.app2))

    it('Deploy app 3', async function() {
        // This operation can take some time
        this.timeout(30 * 1000)
        this.slow(15 * 1000)

        const response = await shared.nodeRequest
            .post('/site/' + sitesData.site3.domain + '/deploy')
            .set('Authorization', shared.auth)
            .send(appData.app3)
            .expect('Content-Type', /json/)
            .expect(200)
        
        assert(response)
        assert.deepStrictEqual(Object.keys(response.body).sort(), ['id', 'site', 'app', 'version', 'deploymentError', 'status', 'time'].sort()) 
        assert(validator.isUUID(response.body.id))
        assert(response.body.time === null)
        assert.strictEqual(response.body.site, shared.siteIds.site3)
        assert.strictEqual(response.body.app, appData.app3.app)
        assert.strictEqual(response.body.version, appData.app3.version)
        assert.strictEqual(response.body.deploymentError, null)
        assert.strictEqual(response.body.status, 'running')

        // Wait for app to be deployed
        await shared.waitForDeployment(sitesData.site3.domain, appData.app3)

        // Add deployed app to the list
        shared.apps[sitesData.site3.domain] = appData.app3

        // App's bundle is in cache
        await shared.checkCacheDirectory(shared.sites, shared.apps)
    })

    it('Check platform data directory', shared.tests.checkDataDirectory(shared.sites, shared.apps))

    it('App 3 is up', shared.tests.checkNginxSite(sitesData.site3, appData.app3))

    it('Status', shared.tests.checkStatus(shared.sites, shared.apps))
})
