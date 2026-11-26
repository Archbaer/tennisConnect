<script lang="ts">
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { ApiError, apiJSON, formatDate } from '#lib/api';
  import { loadSession } from '#lib/session';

  let recoverUntil = '';
  let loading = true;
  let busy = false;
  let error = '';

  onMount(async () => {
    try {
      const result = await apiJSON<{ recover_until: string }>('/api/v1/account/recovery');
      recoverUntil = result.recover_until;
    } catch (problem) {
      if (problem instanceof ApiError && problem.status === 401) {
        error = 'That recovery link has expired. Sign in again to request a new one.';
      } else {
        error = problem instanceof Error ? problem.message : 'We could not load your account.';
      }
    } finally {
      loading = false;
    }
  });

  async function recover() {
    error = '';
    busy = true;
    try {
      await apiJSON('/api/v1/account/recovery', { method: 'POST' });
      await loadSession();
      await goto('/discover');
    } catch (problem) {
      error = problem instanceof Error ? problem.message : 'We could not recover that account.';
    } finally {
      busy = false;
    }
  }

  async function continueDeleting() {
    error = '';
    busy = true;
    try {
      await apiJSON('/api/v1/account/recovery', { method: 'DELETE' });
      await goto('/');
    } catch (problem) {
      error = problem instanceof Error ? problem.message : 'We could not continue deleting that account.';
    } finally {
      busy = false;
    }
  }
</script>

<svelte:head>
  <title>Recover your account · Tennis Connect</title>
</svelte:head>

<section class="page narrow stack">
  {#if loading}
    <p role="status">Checking your account…</p>
  {:else if recoverUntil}
    <p class="score mb-1"><span class="tennis-ball me-2" aria-hidden="true"></span>Scheduled deletion</p>
    <h1>Your account is scheduled for deletion</h1>
    <p>
      It will be deleted on {formatDate(recoverUntil)}. Recovering restores your profile, photos,
      play times and matches. Nothing is visible to other players in the meantime.
    </p>
    <div class="surface stack">
      {#if error}<p class="alert-box mb-0" role="alert">{error}</p>{/if}
      <button class="btn btn-primary" type="button" onclick={recover} disabled={busy}>
        {busy ? 'Working…' : 'Recover my account'}
      </button>
      <button class="btn btn-outline-secondary" type="button" onclick={continueDeleting} disabled={busy}>
        Continue deleting
      </button>
    </div>
  {:else}
    <p class="score mb-1"><span class="tennis-ball me-2" aria-hidden="true"></span>Account recovery</p>
    <h1>This link cannot recover an account</h1>
    <p class="alert-box" role="alert">{error}</p>
    <a class="btn btn-primary align-self-start" href="/">Back to sign in</a>
  {/if}
</section>
