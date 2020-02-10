'use strict'

// Start the code
;(() => {
    // Fetch the info
    nodeInfo()

    // Fetch the status
    nodeStatus()
})()

async function nodeInfo() {
    const response = await fetch('/info')
    const data = await response.json()
    document.getElementById('nodename').innerText = data.hostname || '(no hostname)'
}

async function nodeStatus() {
    const response = await fetch('/status')
    const data = await response.json()
    document.getElementById('nodestatus').innerHTML = JSON.stringify(data, null, 2)
}
