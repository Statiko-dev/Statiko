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
    app1: {
        app: 'app1',
        version: '1',
        contents: {
            'index.html': '3bfa3e40f142c9e6143eab5a9a13bbe5'
        }
    },
    app2: {
        app: 'app2',
        version: '1.0.1',
        contents: {
            'index.html': 'f6bf0230e47135a8245be5a8e49e765f',
            'roquefabio-unsplash.jpg': '3ea45b9bdc5bd2a856df0af23da867cf'
        }
    },
    app2v2: {
        app: 'app2',
        version: '1.2.0',
        contents: {
            'index.html': 'c457a60869554676811f3b0a183aeca6',
            'roquefabio-unsplash.jpg': '3ea45b9bdc5bd2a856df0af23da867cf'
        }
    },
    app3: {
        app: 'app3',
        version: '200',
        contents: {
            '403.html': 'e9f3ffd6f02ff6485585745aeda1f651',
            '404.html': 'dc39a4bece6c7c794063c716af8102c0',
            '_statiko.yaml': '32dbbc01749ce295cd36f9e3aa2f0daa',
            'index.html': 'bdb096fbdc2ca7dc1f23470f9e51384a',
            'mike-erskine-b4AD8zSAozk-unsplash.jpg': '9dbbfd4205fe99c5bd77093b2e034747'
        },
        headers: {
            '404.html': {
                'x-test-header': 'Hello world',
                'expires': '2d',
                'pragma': 'public',
                'cache-control': 'max-age=172800, public',
            },
            'mike-erskine-b4AD8zSAozk-unsplash.jpg': {
                'x-media-type': 'Images',
                'expires': '1M',
                'pragma': 'public',
                'cache-control': 'max-age=2592000, public',
            }
        }
    }
}
