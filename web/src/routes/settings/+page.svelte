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
    <p class="score mb-1"><span class="tennis-ball me-2" aria-hidden="true"></span>Account</p>
    <h1>Settings</h1>
  </div>

  <div class="surface stack">
    <h2><i class="bi bi-person-vcard me-2" aria-hidden="true"></i>Your card</h2>
    <a href="/profile/edit">Edit profile and matching preferences</a>
    <a href="/profile/play-times">Edit play times</a>
    <a href="/profile/photos">Manage photos</a>
    <a href="/onboarding">Change area</a>
  </div>

  <div class="surface stack">
    <h2><i class="bi bi-shield-check me-2" aria-hidden="true"></i>Privacy and safety</h2>
    <p class="muted">
      Your exact area, e-mail and date of birth are never returned by the API. Other players see a
      coarse area only through distance buckets. Instagram is disclosed only inside a live match.
    </p>
    <a href="/report">Report a player</a>
  </div>

  <div class="surface stack">
    <h2><i class="bi bi-database me-2" aria-hidden="true"></i>Your data</h2>
    <a href="/account/export">Download my data</a>
    <a href="/account/delete">Delete account</a>
  </div>

  <div class="surface stack">
    <h2><i class="bi bi-box-arrow-right me-2" aria-hidden="true"></i>Session</h2>
    <button class="btn btn-outline-secondary" onclick={signOut}>Sign out</button>
  </div>
</section>
