<script lang="ts">
  import { goto } from '$app/navigation';
  import { apiJSON } from '#lib/api';

  async function signOut() {
    try {
      await apiJSON('/api/v1/auth/sign-out', { method: 'POST' });
    } finally {
      await goto('/');
    }
  }
</script>

<svelte:head>
  <title>Settings · Tennis Connect</title>
</svelte:head>

<section class="page narrow stack">
  <div>
    <p class="score">Account</p>
    <h1>Settings</h1>
  </div>

  <div class="surface stack">
    <h2>Your card</h2>
    <a href="/profile/edit">Edit profile and matching preferences</a>
    <a href="/profile/play-times">Edit play times</a>
    <a href="/profile/photos">Manage photos</a>
    <a href="/onboarding">Change area</a>
  </div>

  <div class="surface stack">
    <h2>Privacy and safety</h2>
    <p class="muted">
      Your exact area, e-mail and date of birth are never returned by the API. Other players see a
      coarse area only through distance buckets. Instagram is disclosed only inside a live match.
    </p>
    <a href="/report">Report a player</a>
  </div>

  <div class="surface stack">
    <h2>Your data</h2>
    <a href="/account/export">Download my data</a>
    <a href="/account/delete">Delete account</a>
  </div>

  <div class="surface stack">
    <h2>Session</h2>
    <button class="secondary" onclick={signOut}>Sign out</button>
  </div>
</section>
