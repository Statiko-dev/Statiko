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

const {cloneObject} = require('./utils')
const appData = require('./app-data')

const sitesData = {
    site1: {
        domain: 'site1.local',
        tls: {
            type: 'imported',
            cert: 'site1',
            ver: '7f9f2c93860e4c54bc4eea5bb5973ad6',
        },
        aliases: [
            'site1-alias.local',
            'mysite.local'
        ]
    },
    site2: {
        domain: 'site2.local',
        tls: {
            type: 'imported',
            cert: 'site2'
        },
        aliases: [
            'site2-alias.local'
        ]
    },
    site3: {
        domain: 'site3.local',
        tls: {
            type: 'imported',
            cert: 'site3'
        },
        aliases: []
    },

    // Erroring
    exists1: {
        domain: 'site3.local',
        tls: {
            type: 'imported',
            cert: 'site3'
        },
        aliases: ['not-existing.com']
    },
    exists2: {
        domain: 'not-existing.com',
        tls: {
            type: 'imported',
            cert: 'site3'
        },
        aliases: ['site3.local']
    }
}

// site2 with app2 deployed
sitesData.site2app2 = cloneObject(sitesData.site2)
sitesData.site2app2.app = {
    name: appData.app2.app,
    version: appData.app2.version
}

// site3 with app3 deployed
sitesData.site3app3 = cloneObject(sitesData.site3)
sitesData.site3app3.app = {
    name: appData.app3.app,
    version: appData.app3.version
}

// Patch requests for site1
sitesData.site1patch1 = {
    aliases: ['testsite.local']
}
sitesData.site1patch2 = {
    aliases: []
}
sitesData.site1patch3 = {
    tls: {
        cert: 'site3',
        ver: 'dcdc4a65bbc34da981d4949f300e8076'
    }
}

module.exports = sitesData
