<script lang="ts">
  import { goto } from '$app/navigation';
  import { apiJSON } from '#lib/api';

  let confirmed = false;
  let error = '';
  let recoverUntil = '';
  let deleting = false;

  async function remove() {
    error = '';
    deleting = true;
    try {
      const result = await apiJSON<{ recover_until: string }>('/api/v1/account', {
        method: 'DELETE'
      });
      recoverUntil = result.recover_until;
      setTimeout(() => goto('/'), 2500);
    } catch (problem) {
      error = problem instanceof Error ? problem.message : 'We could not delete that account.';
    } finally {
      deleting = false;
    }
  }
</script>

<svelte:head>
  <title>Delete account · Tennis Connect</title>
</svelte:head>

<section class="page narrow stack">
  {#if recoverUntil}
    <p class="score">Scheduled</p>
    <h1>Your account is hidden</h1>
    <p>
      Nothing is visible to other players. Signing in again before
      {new Date(recoverUntil).toLocaleDateString('en-GB')} restores the account. After that it is
      deleted.
    </p>
    <a class="button" href="/">Return to the start</a>
  {:else}
    <p class="score">Careful</p>
    <h1>Delete account</h1>
    <p>
      Your profile, photos, matches and likes are hidden immediately. You have 30 days to recover by
      signing in again. After 30 days the account is deleted.
    </p>
    <p class="muted">
      Download your data first if you want a copy. Reports you made are retained for safety.
    </p>
    {#if error}
      <p class="alert-box" role="alert">{error}</p>
    {/if}
    <label class="confirm">
      <input type="checkbox" bind:checked={confirmed} />
      I understand this hides my account now and deletes it after 30 days.
    </label>
    <div class="row">
      <button class="danger" disabled={!confirmed || deleting} onclick={remove}>
        {deleting ? 'Deleting…' : 'Delete my account'}
      </button>
      <a class="button secondary" href="/settings">Cancel</a>
    </div>
  {/if}
</section>

<style>
  .confirm {
    display: flex;
    align-items: flex-start;
    gap: 0.6rem;
    font-weight: 400;
  }

  .confirm input {
    width: auto;
    min-height: auto;
    margin-top: 0.25rem;
  }
</style>
