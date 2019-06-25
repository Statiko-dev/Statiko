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

// Auth header
const auth = 'hello world'

// Read URLs from env vars
const nodeUrl = process.env.NODE_URL || 'localhost:3000'
const nginxUrl = process.env.NGINX_URL || 'localhost'

// Supertest instances
const nodeRequest = request('https://' + nodeUrl)
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
            expectSites.push(site.id)
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
            expectSites.push(site.id)
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
                (await fsReadFile('/etc/nginx/conf.d/' + sites.site1.id + '.conf', 'utf8')).trim(),
                (await fsReadFile('fixtures/nginx-site1.conf', 'utf8')).trim().replace(/\{\{siteid\}\}/g, sites.site1.id)
            )
        }
    }
}

// Checks that a site is correctly configured on Nginx and it respons to queries
async function checkNginxSite(site, appDeployed) {
    // If an app has been deployed, it should return 200
    // Otherwise, a 403 is expected
    const statusCode = (appDeployed) ? 200 : 403

    // Test the base site, with TLS
    const result = await nginxRequest
        .get('/')
        .set('Host', site.domain)
        .expect(statusCode)
    
    // If an app has been deployed
    if (appDeployed) {
        assert(result.text)
        assert(/text\/html/i.test(result.type))

        // Ensure the contents match
        const indexHash = utils.md5String(result.text)
        assert.strictEqual(indexHash, appDeployed.contents['/'])

        // Check the other files (if any)
        const promises = []
        for (const key in appDeployed.contents) {
            if (appDeployed.contents.hasOwnProperty(key)) {
                if (key == '/') {
                    continue
                }
                
                const p = nginxRequest
                    .get(key)
                    .set('Host', site.domain)
                    .expect(200)
                    .expect(/text\/html/i)
                    .then((res) => {
                        assert(res.text)
                        // Ensure the contents match
                        assert.strictEqual(utils.md5String(res.text), appDeployed.contents[key])
                    })
                promises.push(p)
            }
        }
    }

    // Without TLS, should redirect
    const p = request('http://' + nginxUrl)
        .get('/__hello')
        .set('Host', site.domain)
        .expect(301)
        .expect('Location', 'https://' + site.domain + '/__hello')
    const promises = [p]

    // Test aliases, which should all redirect
    site.aliases.map((el) => {
        // With TLS
        const p1 = request('https://' + nginxUrl)
            .get('/__hello')
            .set('Host', el)
            .expect(301)
            .expect('Location', 'https://' + site.domain + '/__hello')
        
        // Without TLS
        const p2 = request('http://' + nginxUrl)
            .get('/__hello')
            .set('Host', el)
            .expect(301)
            .expect('Location', 'https://' + site.domain + '/__hello')
        
        promises.push(p1, p2)
    })

    // Run in parallel
    await Promise.all(promises)
}

// Checks that the cache directory has the correct data
async function checkCacheDirectory(sites) {
    // TLS Certificates in cache
    if (sites) {
        for (const k in sites) {
            if (!sites.hasOwnProperty(k)) {
                continue
            }

            await utils.fileExists('/data/cache/' + sites[k].tlsCertificate + '.cert.pem')
            await utils.fileExists('/data/cache/' + sites[k].tlsCertificate + '.key.pem')
            await utils.fileExists('/data/cache/' + sites[k].tlsCertificate + '.dhparams.pem')
        }
    }
}

// Waits for an app to be deployed, with a timeout of ~20 seconds
async function waitForDeployment(domain, appData) {
    // Wait 20 seconds max (40 times, every 500ms)
    let t = 40
    while (t--) {
        // Wait 0.5 seconds
        await utils.waitPromise(500)

        const response = await nodeRequest
            .get('/status/')
            .expect('Content-Type', /json/)
            .expect(200)
        assert(response.body)
        assert(response.body.apps)
        assert(Array.isArray(response.body.apps))

        // Check that the app matching site1 is deployed
        let found = null
        for (let i = 0; i < response.body.apps.length; i++) {
            const app = response.body.apps[i]
            if (app && app.domain && app.domain == domain) {
                found = app
                break
            }
        }

        // We should have found one app
        assert(found)

        // Ensure app has been deployed, or keep waiting
        if (found.appName == appData.app && found.appVersion == appData.version && found.updated) {
            return
        }
    }

    // If we're here, app didn't get deployed
    throw Error('Timeout reached: app not deployed')
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

    checkNginxSite: (site, appDeployed) => {
        return async function() {
            await checkNginxSite(site, appDeployed)
        }
    }
}

module.exports = {
    auth,

    nodeUrl,
    nodeRequest,
    nginxUrl,
    nginxRequest,

    checkDataDirectory,
    checkNginxConfig,
    checkNginxSite,
    checkCacheDirectory,
    waitForDeployment,

    tests,

    // Read/write properties

    // Site ids
    siteIds: {},

    // Configured sites
    sites: {}
}
