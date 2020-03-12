<nav id="header" class="bg-white fixed w-full z-10 top-0 shadow">
  <div class="w-full lg:w-3/5 container mx-auto px-2 flex flex-wrap items-center justify-between my-4">
    <div class="pl-4 md:pl-0">
      <span class="flex items-center text-blue-600 text-xl xl:text-2xl no-underline hover:no-underline font-extrabold font-sans">{$nodeInfo.hostname || 'Statiko node'}</span>
    </div>
    <div class="pr-4 md:pr-0">
      <a href="https://statiko.dev/docs">Statiko docs</a>
    </div>
  </div>
</nav>

<script>
import {nodeInfo} from '../stores'
</script>
