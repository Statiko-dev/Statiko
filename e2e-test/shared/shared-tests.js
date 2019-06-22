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
const request = require('supertest')

const utils = require('./utils')

// Promisified methods
const fsReaddir = promisify(fs.readdir)
const fsReadFile = promisify(fs.readFile)

// Supertest instances
// Read URLs from env vars
const nginxUrl = process.env.NGINX_URL || 'localhost'
const nginxRequest = request('https://' + nginxUrl)

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
    assert((await fsReaddir('/data/apps/_default')).length == 1)

    // Sites
    assert.deepStrictEqual((await fsReaddir('/data/sites')).sort(), expectSites.sort())

    for (let i = 0; i < expectSites.length; i++) {
        assert(await utils.folderExists('/data/sites/' + expectSites[i]))
        assert(await utils.fileExists('/data/sites/' + expectSites[i] + '/nginx-error.log'))
        assert(await utils.folderExists('/data/sites/' + expectSites[i] + '/www'))
        if (expectSites[i] == '_default') {
            assert.deepStrictEqual((await fsReaddir('/data/sites/' + expectSites[i])).sort(), ['nginx-error.log', 'www'])
            assert((await fsReaddir('/data/sites/' + expectSites[i] + '/www')).length == 1)
        }
        else {
            assert.deepStrictEqual((await fsReaddir('/data/sites/' + expectSites[i])).sort(), ['nginx-error.log', 'tls', 'www'])
            assert(await utils.folderExists('/data/sites/' + expectSites[i] + '/tls'))
        }
    }
}

// Checks that the Nginx configuration is correct
async function checkNginxConfig(sites) {
    // We always expect the default site and app
    const expectSites = ['_default']

    // Add all expected sites 
    if (sites) {
        Object.entries(sites).forEach((el) => {
            const [, site] = el
            expectSites.push(site.ID)
        })
    }

    // Check the conf.d folder
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

    // Check if the configuration file for all other sites is correct
    if (sites) {
        if (sites.site1) {
            assert.equal(
                (await fsReadFile('/etc/nginx/conf.d/' + sites.site1.ID + '.conf', 'utf8')).trim(),
                (await fsReadFile('fixtures/nginx-site1.conf', 'utf8')).trim().replace(/\{\{siteid\}\}/g, sites.site1.ID)
            )
        }
    }
}

// Checks that a site is correctly configured on Nginx and it respons to queries
async function checkNginxSite(site) {
    // Test the base site, with TLS
    await nginxRequest
        .get('/')
        .set('Host', site.domain)
        .expect(403) // This should fail with a 403

    // Without TLS, should redirect
    await request('http://' + nginxUrl)
        .get('/hello')
        .set('Host', site.domain)
        .expect(301)
        .expect('Location', 'https://' + site.domain + '/hello')

    // Test aliases, which should all redirect
    const promises = site.aliases.map((el) => {
        return Promise.resolve()
            .then(() => {
                // With TLS
                return request('https://' + nginxUrl)
                    .get('/hello')
                    .set('Host', el)
                    .expect(301)
                    .expect('Location', 'https://' + site.domain + '/hello')
            })
            .then(() => {
                // Without TLS
                return request('http://' + nginxUrl)
                    .get('/hello')
                    .set('Host', el)
                    .expect(301)
                    .expect('Location', 'https://' + site.domain + '/hello')
            })
    })
    await Promise.all(promises)
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
            // Check if filesystem is in order
            assert(await utils.folderExists('/etc/nginx'))
            assert(await utils.folderExists('/etc/nginx/conf.d'))
            assert(await utils.fileExists('/etc/nginx/mime.types'))
            assert(await utils.fileExists('/etc/nginx/nginx.conf'))
            assert.deepStrictEqual(
                (await fsReaddir('/etc/nginx')).sort(),
                ['conf.d', 'mime.types', 'nginx.conf']
            )
            
            // Run the rest of the tests checking all config
            await checkNginxConfig(sites)
        }
    },

    checkNginxStatus: () => {
        return function() {
            return nginxRequest
                .get('/')
                .expect(404) // This should fail with a 404
                .expect('Content-Type', 'text/html') // Should return the default app
                .then((response) => {
                    assert(/<title>Welcome to SMPlatform<\/title>/.test(response.text))
                })
        }
    },

    checkNginxSite: (site) => {
        return async function() {
            await checkNginxSite(site)
        }
    }
}

module.exports = {
    checkDataDirectory,
    checkNginxConfig,
    checkNginxSite,
    tests
}
