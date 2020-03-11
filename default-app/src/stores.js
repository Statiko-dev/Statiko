import {readable, writable} from 'svelte/store'

// Node info store
export const nodeInfo = writable({})

// Node status store
const updateStatusStore = (set) => {
    fetch(env.BASE_URL + '/status')
        .then((response) => response.json())
        .then(set)
}
export const nodeStatus = readable(null, (set) => {
    // Request the status
    updateStatusStore(set)
    const interval = setInterval(() => updateStatusStore(set), 1000)
    
    // On un-subscribe, remove the interval
    return () => clearInterval(interval)
})
