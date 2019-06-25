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

const fs = require('fs')
const promisify = require('util').promisify
const crypto = require('crypto')

// Promisified methods
const fsStat = promisify(fs.stat)

/**
 * Returns stats for a path if it exists
 * 
 * @param {string} path - Path to test
 * @returns {fs.Stats|null} stats for the path if it exists, or null
 * @async
 */
const pathExists = async (path) => {
    try {
        const stat = await fsStat(path)
        return stat
    }
    catch (err) {
        if (err.code == 'ENOENT') {
            return null
        }
        else {
            throw err
        }
    }
}

/**
 * Returns true if the file exists
 * 
 * @param {string} path - Path to file
 * @returns {boolean} true if file exists
 * @async
 */
const fileExists = async (path) => {
    const stat = await pathExists(path)
    return stat && stat.isFile()
}

/**
 * Returns true if the folder exists
 * 
 * @param {string} path - Path to folder
 * @returns {boolean} true if folder exists
 * @async
 */
const folderExists = async (path) => {
    const stat = await pathExists(path)
    return stat && stat.isDirectory()
}

/**
 * Returns a promise that resolves after a certain number of ms
 * 
 * @param {number} time - Time to wait in ms
 * @returns {Promise<void>} Promise that resolves after the delay
 */
const waitPromise = (time) => {
    return new Promise((resolve) => {
        setTimeout(resolve, time || 0)
    })
}

/**
 * Returns the MD5 digest of a string
 * 
 * @param {string} str - String to digest
 * @returns {string} MD5 digest
 */
const md5String = (str) => {
    return crypto.createHash('md5').update(str).digest('hex')
}

module.exports = {
    pathExists,
    fileExists,
    folderExists,
    waitPromise,
    md5String
}
