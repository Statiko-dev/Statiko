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

module.exports = {
    site1: {
        domain: 'site1.local',
        tlsCertificate: 'site1',
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
