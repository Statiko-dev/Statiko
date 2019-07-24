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

// Check that the platform has been started correctly
describe('Restore state', function() {
    // The state to create
    const stateData = {
        sites: []
    }

    it('Restore state with no apps', async function() {
        // Add sites; use push rather than changing the variable because of the pointer used in the checkDataDirectory method
        stateData.sites.push(
            cloneObject(sitesData.site1),
            cloneObject(sitesData.site2)
        )

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

    it('Check cache directory', shared.tests.checkCacheDirectory(sitesData.sites, []))

    it('Check data directory', shared.tests.checkDataDirectory(stateData.sites))

    it('Check nginx configuration', shared.tests.checkNginxConfig(stateData.sites))

    it('Test site1 health', shared.tests.checkNginxSite(sitesData.site1))

    it('Test site2 health', shared.tests.checkNginxSite(sitesData.site2))
})
