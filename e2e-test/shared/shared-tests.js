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

'use strict'

const assert = require('assert')
const promisify = require('util').promisify
const fs = require('fs')

const utils = require('./utils')

// Promisified methods
const fsReaddir = promisify(fs.readdir)
const fsReadFile = promisify(fs.readFile)

// This function can be called to check the status of the data directory on the filesystem
// It checks that sites, apps, and certificates are correct
async function checkDataDirectory(sites) {
    // We always expect the default site and app
    const expectSites = ['_default']
    const expectApps = ['_default']

    // Add all expected sites 
    if (sites) {
        Object.entries(sites).forEach((el) => {
            const [, site] = el
            expectSites.push(site.ID)
        })
    }

    // Apps
    assert(await utils.folderExists('/data/apps/_default'))
    assert.deepStrictEqual((await fsReaddir('/data/apps')).sort(), expectApps.sort())
    assert((await fsReaddir('/data/apps/_default')).length == 0)

    // Sites
    assert.deepStrictEqual((await fsReaddir('/data/sites')).sort(), expectSites.sort())

    for (let i = 0; i < expectSites.length; i++) {
        assert(await utils.folderExists('/data/sites/' + expectSites[i]))
        assert(await utils.fileExists('/data/sites/' + expectSites[i] + '/nginx-error.log'))
        assert(await utils.folderExists('/data/sites/' + expectSites[i] + '/www'))
        if (expectSites[i] == '_default') {
            assert.deepStrictEqual((await fsReaddir('/data/sites/' + expectSites[i])).sort(), ['nginx-error.log', 'www'])
            assert((await fsReaddir('/data/sites/' + expectSites[i] + '/www')).length == 0)
        }
        else {
            assert.deepStrictEqual((await fsReaddir('/data/sites/' + expectSites[i])).sort(), ['nginx-error.log', 'tls', 'www'])
            assert(await utils.folderExists('/data/sites/' + expectSites[i] + '/tls'))
        }
    }
}

// Repeated tests
const tests = {
    checkDataDirectory: (sites) => {
        return async function() {
            // Test basic filesystem
            assert(await utils.folderExists('/data'))
            assert(await utils.folderExists('/data/apps'))
            assert(await utils.folderExists('/data/cache'))
            assert(await utils.folderExists('/data/sites'))
            assert.deepStrictEqual(await fsReaddir('/data'), ['apps', 'cache', 'sites'])

            // Check the data directory
            await checkDataDirectory(sites)
        }
    },

    checkConfigDirectory: () => {
        return async function() {
            // Check if directory exists
            assert(await utils.folderExists('/etc/smplatform'))

            // Ensure that the app created the database
            assert(await utils.fileExists('/etc/smplatform/data.db'))
        }
    },

    checkNginxConfig: (sites) => {
        return async function() {
            // We always expect the default site and app
            const expectSites = ['_default']

            // Add all expected sites 
            if (sites) {
                Object.entries(sites).forEach((el) => {
                    const [, site] = el
                    expectSites.push(site.ID)
                })
            }

            // Check if filesystem is in order
            assert(await utils.folderExists('/etc/nginx'))
            assert(await utils.folderExists('/etc/nginx/conf.d'))
            assert(await utils.fileExists('/etc/nginx/mime.types'))
            assert(await utils.fileExists('/etc/nginx/nginx.conf'))
            assert.deepStrictEqual(
                (await fsReaddir('/etc/nginx')).sort(),
                ['conf.d', 'mime.types', 'nginx.conf']
            )
            assert(await utils.fileExists('/etc/nginx/conf.d/_default.conf'))
            assert.deepStrictEqual(
                (await fsReaddir('/etc/nginx/conf.d')).sort(),
                (expectSites.map((el) => el + '.conf')).sort()
            )

            // Check if the configuration for the default site is correct
            assert.equal(
                (await fsReadFile('/etc/nginx/conf.d/_default.conf', 'utf8')).trim(),
                (await fsReadFile('fixtures/nginx-default-site.conf', 'utf8')).trim()
            )
        }
    }
}

module.exports = {
    checkDataDirectory,
    tests
}
