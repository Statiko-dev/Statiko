/* eslint eslint-env mocha  */

const assert = require('assert')
const request = require('supertest')

// Debug
process.env.NODE_TLS_REJECT_UNAUTHORIZED = '0'

// Read URLs from env vars
const nodeUrl = process.env.NODE_URL || 'https://localhost:3000'
const nginxUrl = process.env.NGINX_URL || 'https://localhost'

// Supertest instances
const nodeRequest = request(nodeUrl)
const nginxRequest = request(nginxUrl)

// Check that the platform has been started correctly
describe('SMPlatform health', function() {
    it('Node is up', function() {
        return nodeRequest
            .get('/')
            .expect(404) // This should correctly return 404
    })

    it('Nginx is up', function() {
        return nginxRequest
            .get('/')
            .expect(403) // This should fail with a 403
    })

    it('Get node info from /info', function() {
        return nodeRequest
            .get('/info')
            .expect('Content-Type', /json/)
            .expect(200)
            .then(response => {
                assert.ok(response.body)
                assert(Object.keys(response.body).length == 3)
                assert(response.body.authMethod == 'sharedkey')
                assert.ok(response.body.version) // TODO: Need to validate version
                assert.ok(response.body.hostname)
            })
    })

    it('Get node info via proxy', function() {
        return nginxRequest
            .get('/info')
            .expect('Content-Type', /json/)
            .expect(200)
            .then(response => {
                assert.ok(response.body)
                assert(Object.keys(response.body).length == 3)
                assert(response.body.authMethod == 'sharedkey')
                assert.ok(response.body.version) // TODO: Need to validate version
                assert.ok(response.body.hostname)
            })
    })
})
