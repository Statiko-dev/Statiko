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

const {cloneObject} = require('./utils')
const appData = require('./app-data')

const sitesData = {
    site1: {
        domain: 'site1.local',
        tlsCertificate: 'site1',
        tlsCertificateVersion: '72cd150c5f394bd190749cdb22d0f731',
        clientCaching: true,
        aliases: [
            'site1-alias.local',
            'mysite.local'
        ]
    },
    site2: {
        domain: 'site2.local',
        tlsCertificate: 'site2',
        clientCaching: false,
        aliases: [
            'site2-alias.local'
        ]
    },
    site3: {
        domain: 'site3.local',
        tlsCertificate: 'site3',
        clientCaching: true,
        aliases: []
    },

    // Erroring
    exists1: {
        domain: 'site3.local',
        tlsCertificate: 'site3',
        clientCaching: true,
        aliases: ['not-existing.com']
    },
    exists2: {
        domain: 'not-existing.com',
        tlsCertificate: 'site3',
        clientCaching: true,
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

module.exports = sitesData
