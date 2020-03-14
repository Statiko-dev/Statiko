<div class="flex flex-wrap cursor-pointer p-3 m-2 leading-normal rounded shadow bg-white {site.error ? ' border border-red-600' : ''}" on:click={() => expanded = !expanded}>
    <h2 class="block w-1/3 text-lg flex-grow {site.error ? ' text-red-600' : ''}">{site.domain}</h2>
    <p class="block w-1/3 text-gray-500 text-lg flex-grow">{!expanded && site.app || ''}</p>
    {#if !site.app}
        <p class="block text-lg text-yellow-600 flex-grow-0">
            <i class="fa fa-circle-o" aria-hidden="true" title="No app deployed to this site"></i>
            <span class="sr-only">No app deployed to this site</span>
        </p>
    {:else if !site.error}
        <p class="block text-lg text-green-600 flex-grow-0">
            <i class="fa fa-check-circle-o" aria-hidden="true" title="Site is healthy"></i>
            <span class="sr-only">Site is healthy</span>
        </p>
    {:else}
        <p class="block text-lg text-red-600 flex-grow-0">
            <i class="fa fa-times-circle-o" aria-hidden="true" title="Site's health is degraded"></i>
            <span class="sr-only">Site's health is degraded</span>
        </p>
    {/if}

    {#if expanded}
        <ul class="w-full flex-shrink-0 mt-2 ml-2">
            {#if site.error}
                <li class="text-red-600"><strong>Error: </strong> {site.error}</li>
            {/if}
            <li><strong>App:</strong> {site.app || 'No app deployed'}</li>
            {#if site.status && site.time}
                <li><strong>Status code:</strong> {site.status}</li>
                <li><strong>Response size:</strong> {site.size}</li>
                <li><strong>Last check:</strong> {FormatDate(site.time)}</li>
            {/if}
        </ul>
    {/if}
</div>

<script>
import {FormatDate} from '../utils'

// Props for the view
export let expanded = false
export let site = {}
</script>