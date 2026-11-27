<script lang="ts">
  import { tick } from 'svelte';
  import { apiJSON, type Profile } from '#lib/api';

  export let onSaved: (() => void) | undefined = undefined;

  let dialog: HTMLDialogElement;
  let heading: HTMLHeadingElement;
  let profile: Profile | null = null;
  let genderPreference: Profile['gender_preference'] = 'all';
  let minAge = 18;
  let maxAge = 120;
  let skillTolerance: 0 | 1 = 1;
  let loading = false;
  let saving = false;
  let error = '';

  export async function open() {
    error = '';
    loading = true;
    if (!dialog.open) dialog.showModal();
    await tick();
    heading?.focus();
    try {
      profile = await apiJSON<Profile>('/api/v1/profile');
      genderPreference = profile.gender_preference;
      minAge = profile.min_age;
      maxAge = profile.max_age;
      skillTolerance = profile.skill_tolerance;
    } catch (problem) {
      error = problem instanceof Error ? problem.message : 'We could not load your matching settings.';
    } finally {
      loading = false;
    }
  }

  function close() {
    dialog.close();
  }

  async function save(event: SubmitEvent) {
    event.preventDefault();
    if (!profile) return;
    error = '';
    saving = true;
    try {
      await apiJSON('/api/v1/profile', {
        method: 'PUT',
        body: JSON.stringify({
          ...profile,
          gender_preference: genderPreference,
          min_age: Number(minAge),
          max_age: Number(maxAge),
          skill_tolerance: Number(skillTolerance)
        })
      });
      dialog.close();
      onSaved?.();
    } catch (problem) {
      error = problem instanceof Error ? problem.message : 'We could not save those settings.';
    } finally {
      saving = false;
    }
  }
</script>

<dialog bind:this={dialog} class="matching-dialog" aria-labelledby="matching-title" onclose={() => (error = '')}>
  <div class="dialog-head">
    <div>
      <p class="score mb-1">Matching</p>
      <h2 id="matching-title" class="mb-0" tabindex="-1" bind:this={heading}>Who you'd like to meet</h2>
    </div>
    <button class="icon-button dark" type="button" onclick={close} aria-label="Close">
      <i class="bi bi-x-lg" aria-hidden="true"></i>
    </button>
  </div>

  {#if loading}
    <p role="status">Loading matching settings…</p>
  {:else if profile}
    <form class="stack" onsubmit={save}>
      <label>
        Show me
        <select class="form-select" bind:value={genderPreference}>
          <option value="all">Everyone</option>
          <option value="women">Women</option>
          <option value="men">Men</option>
        </select>
      </label>
      <fieldset>
        <legend>Age range</legend>
        <div class="grid two">
          <label>
            Minimum
            <input class="form-control" type="number" min="18" max="120" bind:value={minAge} />
          </label>
          <label>
            Maximum
            <input class="form-control" type="number" min="18" max="120" bind:value={maxAge} />
          </label>
        </div>
      </fieldset>
      <label>
        Skill match
        <select class="form-select" bind:value={skillTolerance}>
          <option value={0}>Exact level only</option>
          <option value={1}>Include the adjacent level</option>
        </select>
      </label>
      {#if error}<p class="alert-box mb-0" role="alert">{error}</p>{/if}
      <div class="dialog-actions">
        <button class="btn btn-primary" type="submit" disabled={saving}>{saving ? 'Saving…' : 'Save matching'}</button>
        <button class="btn btn-outline-secondary" type="button" onclick={close}>Cancel</button>
      </div>
    </form>
  {:else if error}
    <p class="alert-box mb-0" role="alert">{error}</p>
    <button class="btn btn-outline-secondary" type="button" onclick={close}>Close</button>
  {/if}
</dialog>

<style>
  .matching-dialog {
    width: min(100% - 2rem, 32rem);
    max-height: min(90dvh, 48rem);
    margin: auto;
    padding: 1.5rem;
    border: 1px solid var(--line);
    border-radius: var(--radius-control);
    background: var(--surface);
    color: var(--ink);
    box-shadow: 0 28px 80px rgb(6 40 31 / 28%);
    overflow-y: auto;
    overscroll-behavior: contain;
  }

  .matching-dialog::backdrop {
    background: rgb(6 40 31 / 55%);
    backdrop-filter: blur(3px);
  }

  .dialog-head {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 1rem;
    margin-bottom: 1rem;
  }

  .dialog-head h2:focus {
    outline: none;
  }

  .dialog-actions {
    display: flex;
    flex-wrap: wrap;
    gap: 0.5rem;
  }

  .dialog-actions .btn {
    flex: 1 1 10rem;
  }

  fieldset {
    margin: 0;
    padding: 0;
    border: 0;
  }

  legend {
    font-weight: 700;
    margin-bottom: 0.35rem;
  }

  @media (max-width: 520px) {
    .matching-dialog {
      width: 100%;
      max-height: 92dvh;
      margin: auto 0 0;
      border-radius: var(--radius-control) var(--radius-control) 0 0;
      padding: 1.25rem 1.25rem calc(1.25rem + env(safe-area-inset-bottom));
    }
  }
</style>
