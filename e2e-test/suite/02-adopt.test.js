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

const shared = require('../shared/shared-tests')

// Check that the platform has been started correctly
describe('Adopt node', function() {
    it('Adopt node', async function() {
        // This operation can take some time
        this.timeout(10 * 1000)
        this.slow(5 * 1000)

        const response = await shared.nodeRequest
            .post('/adopt')
            .set('Authorization', shared.auth)
            .expect('Content-Type', /json/)
            .expect(200)

        assert(response.body)
        assert.deepStrictEqual(response.body, {message: 'adopted'})
    })

    it('Check platform data directory', shared.tests.checkDataDirectory(shared.sites))

    it('Check platform config directory', shared.tests.checkConfigDirectory())

    it('Check nginx configuration', shared.tests.checkNginxConfig())
})
