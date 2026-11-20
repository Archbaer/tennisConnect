<script lang="ts">
  import { onMount } from 'svelte';
  import { apiJSON, reportCategories } from '#lib/api';

  let target = '';
  let category = 'harassment';
  let details = '';
  let error = '';
  let submitted = false;
  let submitting = false;

  onMount(() => {
    target = new URL(window.location.href).searchParams.get('target') ?? '';
  });

  async function submit(event: SubmitEvent) {
    event.preventDefault();
    error = '';
    submitting = true;
    try {
      await apiJSON('/api/v1/reports', {
        method: 'POST',
        body: JSON.stringify({ target_id: target, category, details })
      });
      submitted = true;
    } catch (problem) {
      error = problem instanceof Error ? problem.message : 'We could not submit that report.';
    } finally {
      submitting = false;
    }
  }
</script>

<svelte:head>
  <title>Report a player · Tennis Connect</title>
</svelte:head>

<section class="page narrow stack">
  {#if submitted}
    <p class="score">Report received</p>
    <h1>Thank you for telling us</h1>
    <p>
      A reviewer will see this report. The other player is never told who reported them, and we do
      not show report counts or moderation state.
    </p>
    <p class="notice">Reports are reviewed against a 72-hour target.</p>
    <a class="button" href="/discover">Back to Discover</a>
  {:else}
    <p class="score">Safety</p>
    <h1>Report a player</h1>
    <p class="muted">Never include your own contact details. The other player is not told who reported them.</p>
    {#if error}
      <p class="alert-box" role="alert">{error}</p>
    {/if}
    <form class="surface stack" onsubmit={submit}>
      <fieldset>
        <legend>What happened?</legend>
        <div class="stack">
          {#each reportCategories as [value, label]}
            <label class="radio">
              <input type="radio" name="category" value={value} bind:group={category} />
              {label}
            </label>
          {/each}
        </div>
      </fieldset>
      <label>
        Details (optional)
        <textarea maxlength="500" bind:value={details}></textarea>
        <small>{details.length}/500</small>
      </label>
      <button type="submit" disabled={submitting || !target}>
        {submitting ? 'Submitting…' : 'Submit report'}
      </button>
      <a class="button secondary" href="/discover">Cancel</a>
    </form>
  {/if}
</section>

<style>
  .radio {
    display: flex;
    align-items: center;
    gap: 0.6rem;
    font-weight: 400;
  }

  .radio input {
    width: auto;
    min-height: auto;
  }

  fieldset {
    border: 0;
    padding: 0;
    margin: 0;
  }

  legend {
    font-weight: 700;
    margin-bottom: 0.5rem;
  }
</style>
