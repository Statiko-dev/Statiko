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
        assert.deepStrictEqual(Object.keys(response.body).sort(), ['id', 'createdAt', 'updatedAt', 'site', 'app', 'version', 'deploymentError', 'status'].sort()) 
        assert(validator.isUUID(response.body.id))
        assert(validator.isISO8601(response.body.createdAt, {strict: true}))
        assert(validator.isISO8601(response.body.updatedAt, {strict: true}))
        assert.strictEqual(response.body.site, shared.siteIds.site1)
        assert.strictEqual(response.body.app, appData.app1.app)
        assert.strictEqual(response.body.version, appData.app1.version)
        assert.strictEqual(response.body.deploymentError, null)
        assert.strictEqual(response.body.status, 'running')

        // Wait for app to be deployed
        await shared.waitForDeployment(sitesData.site1.domain, appData.app1)
    })

    it('App 1 is up', shared.tests.checkNginxSite(sitesData.site1, appData.app1))
})
