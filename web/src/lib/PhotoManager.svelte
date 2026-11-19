<script lang="ts">
  import { apiJSON, type Photo } from '#lib/api';
  import { onMount } from 'svelte';

  export let photos: Photo[] = [];
  export let onChange: (() => void) | undefined = undefined;

  let error = '';
  let busy = false;
  let fileInput: HTMLInputElement;

  onMount(refresh);

  async function refresh() {
    try {
      const result = await apiJSON<{ photos: Photo[] }>('/api/v1/photos');
      photos = result.photos;
      onChange?.();
    } catch {
      // A missing storage configuration is reported when an upload is attempted.
    }
  }

  async function upload(event: Event) {
    const input = event.currentTarget as HTMLInputElement;
    const file = input.files?.[0];
    if (!file) return;
    error = '';
    busy = true;
    try {
      const slot = await apiJSON<{ photo_id: string; upload_url: string }>(
        '/api/v1/photos/upload-url',
        { method: 'POST' }
      );
      const put = await fetch(slot.upload_url, {
        method: 'PUT',
        headers: { 'Content-Type': file.type },
        body: file
      });
      if (!put.ok) {
        throw new Error('The private photo upload failed. Check the storage configuration.');
      }
      await apiJSON(`/api/v1/photos/${slot.photo_id}/process`, { method: 'POST' });
      await refresh();
    } catch (problem) {
      error = problem instanceof Error ? problem.message : 'We could not process that photo.';
    } finally {
      busy = false;
      if (fileInput) fileInput.value = '';
    }
  }
</script>

<section class="surface stack" aria-labelledby="photos-title">
  <div>
    <p class="score">Up to 3 photos</p>
    <h2 id="photos-title">Photos</h2>
    <p class="muted">
      JPEG, PNG or WebP, up to 5 MB. A checked photo is what makes you visible in Discover.
    </p>
  </div>

  {#if photos.length}
    <ul class="photo-list">
      {#each photos as photo (photo.photo_id)}
        <li class="row spread">
          <span>Photo {photo.display_order}</span>
          <span class:alert={photo.status === 'rejected'} class="chip">{photo.status}</span>
        </li>
      {/each}
    </ul>
  {:else}
    <p class="notice">No photos yet. Add one to enter Discover after review.</p>
  {/if}

  {#if error}
    <p class="alert-box" role="alert">{error}</p>
  {/if}

  <label>
    Add a photo
    <input
      bind:this={fileInput}
      type="file"
      accept="image/jpeg,image/png,image/webp"
      onchange={upload}
      disabled={busy || photos.length >= 3}
    />
    <small>{busy ? 'Checking the photo…' : 'The server checks the image and strips location metadata.'}</small>
  </label>
</section>

<style>
  .photo-list {
    display: grid;
    gap: 0.5rem;
    padding: 0;
    margin: 0;
    list-style: none;
  }

  .photo-list li {
    border: 1px solid var(--rule);
    border-radius: 9px;
    padding: 0.6rem 0.75rem;
  }
</style>
