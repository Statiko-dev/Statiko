import {readable, writable} from 'svelte/store'

// Interval for refreshing the status, in seconds
const statusRefreshInterval = 120

// Node info store
export const nodeInfo = writable({})

// Node status store
const updateStatusStore = (set) => {
    let responseDate = null
    fetch(env.BASE_URL + '/status')
        .then((response) => {
            responseDate = new Date(response.headers.get('date'))
            return response.json()
        })
        .then((data) => {
            data._responseDate = responseDate
            return data
        })
        .then(set)
}
export const nodeStatus = readable(null, (set) => {
    // Request the status
    updateStatusStore(set)
    const interval = setInterval(() => updateStatusStore(set), statusRefreshInterval * 1000)
    
    // On un-subscribe, remove the interval
    return () => clearInterval(interval)
})
