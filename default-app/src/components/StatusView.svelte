{#if !$nodeStatus}
    <p><i class="fa fa-spinner fa-spin fa-fw" aria-hidden="true"></i> Requesting node status…</p>
{:else}
    <div class="flex">
        <div class="p-3 m-2 leading-normal rounded shadow bg-white w-1/3">
            <h2 class="text-xl mb-1">Nginx server</h2>
            {#if $nodeStatus.nginx && $nodeStatus.nginx.running}
                <p>Runnning</p>
            {:else}
                <p>Not runnning</p>
            {/if}
        </div>
        <div class="p-3 m-2 leading-normal rounded shadow bg-white w-1/3">
            <h2 class="text-xl mb-1">Sync</h2>
            {#if $nodeStatus.sync && $nodeStatus.sync.running}
                <p>Currently syncing</p>
            {:else}
                <p>Sync completed</p>
                {#if $nodeStatus.sync.lastSync}
                    <p>Last sync: {new Date($nodeStatus.sync.lastSync).toString()}</p>
                {:else}
                    <p>Last sync: unavailable</p>
                {/if}
            {/if}
        </div>
        <div class="p-3 m-2 leading-normal rounded shadow bg-white w-1/3">
            <h2 class="text-xl mb-1">Store</h2>
            {#if $nodeStatus.store && $nodeStatus.store.healthy}
                <p>Healthy</p>
            {:else}
                <p>Degraded</p>
            {/if}
        </div>
    </div>
    <pre>{JSON.stringify($nodeStatus, null, 2)}</pre>
{/if}

<script>
// Stores
import {nodeStatus} from '../stores'
</script>