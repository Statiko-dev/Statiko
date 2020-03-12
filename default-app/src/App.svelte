<Navbar />
<div class="container w-full lg:w-3/5 px-2 pt-10 lg:pt-10 mt-10">
    {#await p}
        <Loading message="Connecting to node…" />
    {:then r}
        <StatusView />
    {:catch err}
        <p>Could not load node info.</p>
        <pre>{err}</pre>
    {/await}
    <Footer />
</div>

<script>
// Components
import Footer from './components/Footer.svelte'
import Loading from './components/Loading.svelte'
import Navbar from './components/Navbar.svelte'
import StatusView from './components/StatusView.svelte'

// Fetch node info
import {nodeInfo} from './stores'
const p = fetch(env.BASE_URL + '/info')
    .then((response) => response.json())
    .then((info) => {
        nodeInfo.set(info)
        document.title = info.hostname ?
            info.hostname + ' | Statiko node' :
            'Statiko node'
    })
</script>
