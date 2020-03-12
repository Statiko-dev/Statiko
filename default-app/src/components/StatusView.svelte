{#if !$nodeStatus}
    <Loading message="Requesting node status…" />
{:else}
    {#if $nodeStatus.nginx && $nodeStatus.nginx.running && $nodeStatus.store && $nodeStatus.store.healthy && $nodeStatus.sync && !$nodeStatus.sync.syncError}
        <div class="p-3 my-2 leading-normal rounded shadow w-5/6 md:w-2/3 mx-auto bg-white border border-green-600 text-green-600"><i class="fa fa-check" aria-hidden="true"></i> Node is healthy</div>
    {:else}
        <div class="p-3 my-2 leading-normal rounded shadow w-5/6 md:w-2/3 mx-auto bg-white border border-red-600 text-red-600"><i class="fa fa-exclamation-triangle" aria-hidden="true"></i> Node's health is degraded</div>
    {/if}

    <h1 class="text-2xl">Node status</h1>
    <div class="flex flex-wrap md:flex-no-wrap mb-3">
        <div class="p-3 m-2 leading-normal rounded shadow bg-white flex-shrink w-full md:w-1/3">
            <h2 class="text-lg mb-1 mb-2"><i class="fa fa-server" aria-hidden="true"></i> Nginx server</h2>
            {#if $nodeStatus.nginx && $nodeStatus.nginx.running}
                <p class="text-green-600"><i class="fa fa-check-circle-o" aria-hidden="true"></i> Runnning</p>
            {:else}
                <p class="text-red-600"><i class="fa fa-times-circle-o" aria-hidden="true"></i> Not runnning</p>
            {/if}
        </div>
        <div class="p-3 m-2 leading-normal rounded shadow bg-white w-full md:w-1/3">
            <h2 class="text-lg mb-1 mb-2"><i class="fa fa-refresh" aria-hidden="true"></i> Sync</h2>
            {#if !$nodeStatus.sync || $nodeStatus.sync.syncError}
                <p class="text-red-600 mb-1"><i class="fa fa-times-circle-o" aria-hidden="true"></i> Sync error</p>
                <p class="text-xs mb-1 text-red-600">Error: {($nodeStatus.sync && $nodeStatus.sync.syncError) || 'Unspecified error'}</p>
                {#if $nodeStatus.sync && $nodeStatus.sync.lastSync}
                    <p class="text-xs">Last sync:<br/>{FormatDate($nodeStatus.sync.lastSync)}</p>
                {/if}
            {:else if $nodeStatus.sync.running}
                <p class="text-blue-600"><i class="fa fa-circle-o" aria-hidden="true"></i> Currently syncing</p>
            {:else}
                <p class="text-green-600 mb-1"><i class="fa fa-check-circle-o" aria-hidden="true"></i> Sync completed</p>
                {#if $nodeStatus.sync.lastSync}
                    <p class="text-xs">Last sync:<br/>{FormatDate($nodeStatus.sync.lastSync)}</p>
                {:else}
                    <p class="text-xs">Last sync:<br/>unavailable</p>
                {/if}
            {/if}
        </div>
        <div class="p-3 m-2 leading-normal rounded shadow bg-white w-full md:w-1/3">
            <h2 class="text-lg mb-1 mb-2"><i class="fa fa-database" aria-hidden="true"></i> Store</h2>
            {#if $nodeStatus.store && $nodeStatus.store.healthy}
                <p class="text-green-600"><i class="fa fa-check-circle-o" aria-hidden="true"></i> Healthy</p>
            {:else}
                <p class="text-red-600"><i class="fa fa-times-circle-o" aria-hidden="true"></i> Degraded</p>
            {/if}
        </div>
    </div>
    <h1 class="text-2xl mb-2">Sites and apps</h1>
    {#if $nodeStatus.health && $nodeStatus.health.length}
        {#each $nodeStatus.health as el}
            <SiteStatus site={el} />
        {/each}
    {:else}
        <p class="mb-2">This node does not have any site configured. <br/><a href="https://statiko.dev/docs">Read the docs</a> to get started.</p>
    {/if}
    <p class="mt-8 mb-2 text-sm">Last update: {FormatDate($nodeStatus._responseDate)}</p>
{/if}

<script>
import {FormatDate} from '../utils'
import {nodeStatus} from '../stores'

import Loading from './Loading.svelte'
import SiteStatus from './SiteStatus.svelte'
</script>