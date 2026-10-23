<script>
  import Cover from './Cover.svelte'
  import Link from './Link.svelte'
  import { fmtCount } from '../lib/format.js'

  let { artist } = $props()
</script>

<Link href={`/artists/${artist.id}`} class="artist">
  <div class="avatar">
    <Cover id={artist.coverArt || artist.id} alt={artist.name} rounded="rounded-full" class="fill" sizes="(max-width: 860px) 40vw, 200px" />
  </div>
  <div class="name clamp-1">{artist.name}</div>
  <div class="sub">{fmtCount(artist.albumCount, '张专辑')}</div>
</Link>

<style>
  :global(.artist) {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 4px;
    text-decoration: none;
    color: var(--text);
    text-align: center;
  }
  .avatar {
    width: 100%;
    aspect-ratio: 1;
    border-radius: 50%;
    overflow: hidden;
    position: relative;
    box-shadow: 0 22px 50px -26px rgba(0, 0, 0, 0.85), 0 0 0 1px var(--line);
    transition: transform 0.5s var(--ease), box-shadow 0.5s var(--ease);
  }
  :global(.artist):hover .avatar {
    transform: translateY(-4px);
    box-shadow: 0 30px 60px -26px rgba(0, 0, 0, 0.9), 0 0 0 1px var(--line-strong);
  }
  .avatar :global(.fill) {
    width: 100%;
    height: 100%;
    display: block;
    transform: scale(1.42);
    transition: transform 0.7s var(--ease);
  }
  :global(.artist):hover .avatar :global(.fill) {
    transform: scale(1.52);
  }
  .name {
    margin-top: 12px;
    font-size: 13px;
    font-weight: 500;
    max-width: 100%;
  }
  .sub {
    font-size: 11px;
    color: var(--text-3);
  }
</style>
