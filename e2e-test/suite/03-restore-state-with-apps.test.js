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
const shared = require('../shared/shared-tests')
const sitesData = require('../shared/sites-data')
const appData = require('../shared/app-data')

// Check that the platform has been started correctly
describe('Restore state with apps', function() {
    // The state to create
    const stateData = {
        sites: []
    }

    // Apps deployed
    const apps = [appData.app2, appData.app3]

    it('Restore state with apps', async function() {
        // Add sites; use push rather than changing the variable because of the pointer used in the checkDataDirectory method
        // Compared to before, we've removed site1 and added site3
        stateData.sites.push(
            cloneObject(sitesData.site2),
            cloneObject(sitesData.site3)
        )

        // Add apps
        stateData.sites[0].app = {
            name: appData.app2.app,
            version: appData.app2.version
        }
        stateData.sites[1].app = {
            name: appData.app3.app,
            version: appData.app3.version
        }

        // Request
        const response = await shared.nodeRequest
            .post('/state')
            .set('Authorization', shared.auth)
            .send(stateData)
            .expect(204)

        // Tests
        assert(response.text.length == 0)
    })

    it('Wait for sync', shared.tests.waitForSync())

    it('Check cache directory', shared.tests.checkCacheDirectory(sitesData.sites, apps))

    it('Check data directory', shared.tests.checkDataDirectory(stateData.sites))

    it('Check nginx configuration', shared.tests.checkNginxConfig(stateData.sites))

    it('Test site2 health', shared.tests.checkNginxSite(sitesData.site2, appData.app2))

    it('Test site3 health', shared.tests.checkNginxSite(sitesData.site3, appData.app3))

    it('Check status', shared.tests.checkStatus(stateData.sites))
})
