{#await p}
    <Loading />
{:then r}
    <Navbar />
    <div class="container w-full lg:w-3/5 px-2 pt-8 lg:pt-16 mt-16">
        <StatusView />
        <Footer />
    </div>
{:catch err}
    <p>Could not load node info.</p>
    <pre>{err}</pre>
{/await}

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
    .then((info) => nodeInfo.set(info))
</script>
