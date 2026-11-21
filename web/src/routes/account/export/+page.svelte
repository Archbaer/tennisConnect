<svelte:head>
  <title>Download your data · Tennis Connect</title>
</svelte:head>

<section class="page narrow stack">
  <div>
    <p class="score">Your data</p>
    <h1>Download your data</h1>
  </div>
  <div class="surface stack">
    <p>Includes your profile, play times, photo list, likes, matches and reports you made.</p>
    <p class="muted">
      Excludes e-mail, date of birth, coordinates, other players' Instagram handles, signed photo
      URLs and reports other people made.
    </p>
    <a class="button" href="/api/v1/account/export" download="tennis-connect-data.json">
      Download JSON
    </a>
    <a class="button secondary" href="/settings">Back to settings</a>
  </div>
</section>
