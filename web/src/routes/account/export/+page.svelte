<script lang="ts">
  import { showToast } from '#lib/toasts';
</script>

<svelte:head>
  <title>Download your data · Tennis Connect</title>
</svelte:head>

<section class="page narrow stack">
  <header class="page-head">
    <div>
      <p class="kicker">Your data</p>
      <h1>Download your data</h1>
    </div>
  </header>
  <div class="sheet-card stack">
    <p class="mb-0">Includes your profile, play times, photo list, likes, matches and reports you made.</p>
    <p class="muted mb-0">
      Excludes e-mail, date of birth, coordinates, other players' Instagram handles, signed photo
      URLs and reports other people made.
    </p>
    <a
      class="btn btn-primary align-self-start"
      href="/api/v1/account/export"
      download="tennis-connect-data.json"
      onclick={() => showToast('Your data export is downloading.')}
    >
      Download JSON
    </a>
    <a class="btn btn-outline-secondary align-self-start" href="/settings">Back to settings</a>
  </div>
</section>
