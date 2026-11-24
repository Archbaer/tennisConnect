<script lang="ts">
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';

  let token = '';
  let birthDate = '';
  let error = '';
  let underage = false;
  let submitting = false;

  onMount(() => {
    token = new URL(window.location.href).searchParams.get('token') ?? '';
    if (!token) {
      error = 'That link is missing or expired. Request a new one.';
    }
  });

  async function register(event: SubmitEvent) {
    event.preventDefault();
    error = '';
    underage = false;
    submitting = true;
    try {
      const response = await fetch('/api/v1/auth/register', {
        method: 'POST',
        credentials: 'include',
        headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
        body: new URLSearchParams({ token, birth_date: birthDate })
      });
      if (response.ok) {
        await goto('/onboarding');
        return;
      }
      if (response.status === 400) {
        underage = true;
        error = 'Tennis Connect is for adults aged 18 and over.';
        return;
      }
      error = 'That link is invalid or expired. Request a new one.';
    } catch {
      error = 'We could not complete registration. Try again.';
    } finally {
      submitting = false;
    }
  }
</script>

<svelte:head>
  <title>Your date of birth · Tennis Connect</title>
</svelte:head>

<section class="page narrow stack">
  {#if underage}
    <p class="score">Adults only</p>
    <h1>You need to be 18 or older</h1>
    <p>
      Tennis Connect is only for adults. We cannot create an account for someone under 18. This
      keeps the community and the reporting process consistent.
    </p>
    <p class="alert-box" role="alert">{error}</p>
    <a class="btn btn-outline-secondary" href="/">Back to the start</a>
  {:else}
    <p class="score mb-1"><span class="tennis-ball me-2" aria-hidden="true"></span>New account</p>
    <h1>Confirm your age</h1>
    <p>We ask once. Your date of birth is never shown to anyone or returned by the API.</p>
    {#if error}
      <p class="alert-box" role="alert">{error}</p>
    {/if}
    <form class="surface stack" onsubmit={register}>
      <label>
        Date of birth
        <input class="form-control" type="date" name="birth_date" required bind:value={birthDate} max={new Date().toISOString().slice(0, 10)} />
      </label>
      <button class="btn btn-primary" type="submit" disabled={submitting || !token}>
        {submitting ? 'Creating account…' : 'Continue'}
      </button>
      <small>You must be 18 or older. This is self-declared, not identity verification.</small>
    </form>
  {/if}
</section>
