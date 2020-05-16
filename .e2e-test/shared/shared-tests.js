/*
Copyright © 2020 Alessandro Segala (@ItalyPaleAle)

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

const path = require('path')
const assert = require('assert')
const {readdir, readFile} = require('fs').promises
const {readFileSync} = require('fs')
const request = require('supertest')
const validator = require('validator')
const yaml = require('js-yaml')

const utils = require('./utils')
const appData = require('./app-data')

// Auth header
const auth = 'hello world'

// Read URLs from env vars
const nodeUrl = process.env.NODE_URL || 'localhost:2265'
const nginxUrl = process.env.NGINX_URL || 'localhost'

// Supertest instances
const nodeRequest = request('https://' + nodeUrl)
const nginxRequest = request('https://' + nginxUrl)

// Load node's config
const nodeConfig = yaml.safeLoad(readFileSync('/etc/statiko/node-config.yaml'))

// Scan a directory recursively
async function* readdirRecursiveGenerator(dir) {
    const list = await readdir(dir, { withFileTypes: true })
    for (const el of list) {
        const res = path.join(dir, el.name)
        if (el.isDirectory()) {
            yield* readdirRecursiveGenerator(res)
        }
        else {
            yield res
        }
    }
}

async function readdirRecursive(dir) {
    const list = []
    for await (const el of readdirRecursiveGenerator(dir)) {
        list.push(el)
    }
    return list
}

// Checks the /status page
async function checkStatus(sites) {
    // Request status
    const response = await nodeRequest
        .get('/status')
        .set('Authorization', auth)
        .expect('Content-Type', /json/)
        .expect(200)
    assert.deepStrictEqual(Object.keys(response.body).sort(), ['health', 'name', 'nginx', 'store', 'sync'])

    // Check the sync object
    assert(Object.keys(response.body.sync).length == 2)
    // When this function is called, there shouldn't be any sync running
    assert.equal(response.body.sync.running, false)
    assert(validator.isISO8601(response.body.sync.lastSync, {strict: true}))
    const lastSync = new Date(response.body.sync.lastSync)
    // Must have run within the last 5 mins
    assert(Date.now() - lastSync.getTime() < 5 * 60 * 1000)

    // Check the nginx object
    assert.deepStrictEqual(response.body.nginx, {running: true})

    // Check the store object
    assert.deepStrictEqual(response.body.store, {healthy: true})

    // Check the name
    assert.deepStrictEqual(response.body.name, 'e2e-test')

    // Function that returns the object for a given site
    const findSite = (domain) => {
        for (const k in sites) {
            if (Object.prototype.hasOwnProperty.call(sites, k)) {
                if (sites[k].domain == domain) {
                    return sites[k]
                }
            }
        }
        return null
    }

    // Check the health object
    if (sites && sites.length) {
        assert(Array.isArray(response.body.health))
        assert(response.body.health.length === sites.length)

        const keys = []
        for (let i = 0; i < response.body.health.length; i++) {
            const el = response.body.health[i]
            assert(el.domain)

            // Look for the corresponding site object
            const s = findSite(el.domain)
            assert(s)

            // Is there an app deployed?
            if (s.app && s.app.name) {
                assert(!el.error)
                assert.deepStrictEqual(Object.keys(el).sort(), ['app', 'domain', 'healthy', 'time'])
                assert(el.app === s.app.name)
                assert(el.healthy)
                assert(validator.isISO8601(el.time, {strict: true}))
            }
            else {
                assert(!el.error)
                assert.deepStrictEqual(Object.keys(el).sort(), ['app', 'domain', 'healthy'])
                assert(el.app === null)
                assert(el.healthy)
            }

            keys.push(el.domain)
        }

        // Check if we had all the correct sites
        assert.deepStrictEqual(
            keys.sort(),
            sites.map(s => s.domain).sort()
        )
    }
}

// This function can be called to check the status of the data directory on the filesystem
// It checks that sites, apps, and certificates are correct
async function checkDataDirectory(sites) {
    // We always expect the default site and app
    const expectSites = ['_default']
    const expectApps = ['_default']

    // Add all expected sites 
    if (sites) {
        sites.map((site) => {
            expectSites.push(site.domain)
            if (site.app && site.app.name) {
                expectApps.push(site.app.name)
            }
        })
    }

    // Sites
    assert.deepStrictEqual((await readdir('/data/sites')).sort(), expectSites.sort())

    for (let i = 0; i < expectSites.length; i++) {
        assert(await utils.folderExists('/data/sites/' + expectSites[i]))
        assert(await utils.fileExists('/data/sites/' + expectSites[i] + '/nginx-error.log'))
        assert(await utils.folderExists('/data/sites/' + expectSites[i] + '/www'))
        if (expectSites[i] == '_default') {
            assert.deepStrictEqual((await readdir('/data/sites/_default')).sort(), ['nginx-error.log', 'www'])
            assert(await utils.fileExists('/data/sites/_default/www/statiko-welcome.html'), 'File statiko-welcome.html not found in default app')
        }
        else {
            assert.deepStrictEqual((await readdir('/data/sites/' + expectSites[i])).sort(), ['nginx-error.log', 'tls', 'www'])
            assert(await utils.folderExists('/data/sites/' + expectSites[i] + '/tls'))
        }
    }

    // Function that returns the app's content
    const appContents = function(find) {
        // Iterate through the apps
        for (const key in appData) {
            if (Object.prototype.hasOwnProperty.call(appData, key)) {
                const str = appData[key].app
                if (str == find) {
                    return appData[key].contents
                }
            }
        }
    }

    // Apps
    assert(await utils.folderExists('/data/apps/_default'))
    assert.deepStrictEqual((await readdir('/data/apps')).sort(), expectApps.sort())

    for (let i = 0; i < expectApps.length; i++) {
        if (expectApps[i] == '_default') {
            assert(await utils.fileExists('/data/apps/_default/statiko-welcome.html'), 'File statiko-welcome.html not found in default app')
        }
        else {
            // Check all files and their md5 hash
            const contents = appContents(expectApps[i])
            assert((await readdirRecursive('/data/apps/' + expectApps[i])).length == Object.keys(contents).length)
            for (const file in contents) {
                if (Object.prototype.hasOwnProperty.call(contents, file)) {
                    const hash = contents[file]
                    const content = await readFile('/data/apps/' + expectApps[i] + '/' + file)
                    assert.strictEqual(hash, utils.md5String(content))
                }
            }
        }
    }
}

// Checks that the Nginx configuration is correct
async function checkNginxConfig(sites) {
    // We always expect the default site and app
    const expectSites = ['_default']

    // Add all expected sites 
    if (sites) {
        sites.map((site) => {
            expectSites.push(site.domain)
        })
    }

    // Check the conf.d folder
    assert(await utils.fileExists('/etc/nginx/conf.d/_default.conf'))
    assert.deepStrictEqual(
        (await readdir('/etc/nginx/conf.d')).sort(),
        (expectSites.map((el) => el + '.conf')).sort()
    )

    // Check if the configuration for the default site is correct
    assert.equal(
        (await readFile('/etc/nginx/conf.d/_default.conf', 'utf8')).trim(),
        (await readFile('fixtures/nginx-default-site.conf', 'utf8')).trim()
    )

    // Check if the configuration file for all other sites is correct
    if (sites) {
        const keys = ['site1.local', 'site2.local', 'site3.local']
        for (const i in keys) {
            const k = keys[i]
            if (expectSites.indexOf(k) != -1) {
                assert.equal(
                    (await readFile('/etc/nginx/conf.d/' + k + '.conf', 'utf8')).trim(),
                    (await readFile('fixtures/nginx-' + k + '.conf', 'utf8')).trim()
                    , 'Error with site ' + k)
            }
        }
    }
}

// Checks that a site is correctly configured on Nginx and it respons to queries
async function checkNginxSite(site, appDeployed) {
    // If an app has been deployed, it should return 200
    // Otherwise, a 403 is expected
    const statusCode = appDeployed ? 200 : 403

    // Test the base site, with TLS
    const result = await nginxRequest
        .get('/')
        .set('Host', site.domain)
        .expect(statusCode)

    const promises = []
    
    // If an app has been deployed
    if (appDeployed) {
        assert(result.text)
        assert(/text\/html/i.test(result.type))

        // Ensure the contents match
        const indexHash = utils.md5String(result.text)
        assert.strictEqual(indexHash, appDeployed.contents['index.html'])

        // Check the other files (if any)
        for (const key in appDeployed.contents) {
            if (Object.prototype.hasOwnProperty.call(appDeployed.contents, key)) {
                if (key == 'index.html') {
                    continue
                }

                // If the key is the manifest file, expect a 404
                // Also, certain paths are denied, so we can expect a 404 there too
                if (key == '_statiko.yaml' ||
                    (appDeployed.expect404 && appDeployed.expect404.includes(key))) {
                    const p = nginxRequest
                        .get('/' + key)
                        .set('Host', site.domain)
                        .expect(404)
                    promises.push(p)
                    continue
                }
                
                let p = nginxRequest
                    .get('/' + key)
                    .set('Host', site.domain)
                    .expect(200)
                if (appDeployed.headers && appDeployed.headers[key]) {
                    for (let header in appDeployed.headers[key]) {
                        if (!Object.prototype.hasOwnProperty.call(appDeployed.headers[key], header)) {
                            continue
                        }

                        header = header.toLowerCase()
                        if (header == 'expires') {
                            // Just check that it's a RFC-2822-formatted date
                            // Based on https://stackoverflow.com/q/9352003/192024
                            p = p.expect(header, /^(?:(Sun|Mon|Tue|Wed|Thu|Fri|Sat),\s+)?(0[1-9]|[1-2]?[0-9]|3[01])\s+(Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec)\s+(19[0-9]{2}|[2-9][0-9]{3})\s+(2[0-3]|[0-1][0-9]):([0-5][0-9])(?::(60|[0-5][0-9]))?\s+([-\+][0-9]{2}[0-5][0-9]|(?:UT|GMT|(?:E|C|M|P)(?:ST|DT)|[A-IK-Z]))(\s+|\(([^\(\)]+|\\\(|\\\))*\))*$/)
                        }
                        else {
                            p = p.expect(header, appDeployed.headers[key][header])
                        }
                    }
                }
                p = p.then((res) => {
                    assert(res.body && ((res.text && res.text.length) || (res.body && res.body.length)))
                    // Ensure the contents match
                    const body = (res.body && res.body.length) ? res.body : res.text
                    assert.strictEqual(utils.md5String(body), appDeployed.contents[key])
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
    promises.push(p)

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
async function checkCacheDirectory(sites, apps) {
    // Cached apps' bundles
    if (apps) {
        for (const k in apps) {
            if (!Object.prototype.hasOwnProperty.call(apps, k)) {
                continue
            }

            await utils.fileExists('/data/cache/' + apps[k].app + '.tar.bz2')
        }
    }
}

// Waits until state syncs have completed
async function waitForSync() {
    // Wait for 0.5 seconds regardless
    await utils.waitPromise(500)

    // Request the status
    let running = true
    while (running) {
        const response = await nodeRequest
            .get('/status')
            .set('Authorization', auth)
            .expect('Content-Type', /json/)
            .expect(200)
        if (!response || !response.body || !response.body.sync) {
            throw Error('Invalid response: missing the sync object')
        }

        if (response.body.sync.running === true) {
            await utils.waitPromise(500)
        }
        else {
            running = false
        }
    }
}

// Repeated tests
const tests = {
    checkStatus: (sites) => {
        return function() {
            return checkStatus(sites)
        }
    },

    checkDataDirectory: (sites) => {
        return async function() {
            // This operation can take some time
            this.timeout(8 * 1000)
            this.slow(4 * 1000)
    
            // Test basic filesystem
            assert(await utils.folderExists('/data'))
            assert(await utils.folderExists('/data/apps'))
            assert(await utils.folderExists('/data/cache'))
            assert(await utils.folderExists('/data/misc'))
            assert(await utils.folderExists('/data/sites'))
            assert.deepStrictEqual(await readdir('/data'), ['apps', 'cache', 'misc', 'sites'])

            // Check the data directory
            await checkDataDirectory(sites)
        }
    },

    checkCacheDirectory: (sites, apps) => {
        return function() {    
            // Check the data directory
            return checkCacheDirectory(sites, apps)
        }
    },

    checkConfigDirectory: () => {
        return async function() {
            // Check if directory exists
            assert(await utils.folderExists('/etc/statiko'))

            // Check for config file if storing on file
            if ((process.env.STATE_STORE && process.env.STATE_STORE == 'file') || (nodeConfig && nodeConfig.state && nodeConfig.state.store == 'file')) {
                assert(await utils.fileExists('/etc/statiko/state.json'))
            }

            // Check for dhparams
            assert(await utils.fileExists('/data/misc/dhparams.pem'))
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
                (await readdir('/etc/nginx')).sort(),
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
                    assert(/<title>Statiko node<\/title>/.test(response.text))
                })
        }
    },

    checkNginxSite: (site, appDeployed) => {
        return async function() {
            await checkNginxSite(site, appDeployed)
        }
    },

    // Similar to the checkNginxSite function but for a site that's not in the data structure yet
    checkNginxSiteIndex: (sites, index, appDeployed) => {
        return async function() {
            await checkNginxSite(sites[index], appDeployed)
        }
    },

    waitForSync: () => {
        return function() {
            // This operation can take some time
            this.timeout(30 * 1000)
            this.slow(20 * 1000)

            return waitForSync()
        }
    }
}

module.exports = {
    auth,

    nodeUrl,
    nodeRequest,
    nginxUrl,
    nginxRequest,

    checkStatus,
    checkDataDirectory,
    checkNginxConfig,
    checkNginxSite,
    checkCacheDirectory,
    waitForSync,

    tests
}
