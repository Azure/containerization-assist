import { createTool, z } from './_tool-factory.js';

export default createTool({
  name: 'tag_image',
  title: 'Tag Docker Image',
  description: 'Tag the Docker image with version and metadata',
  inputSchema: {
    session_id: z.string().describe('Session ID from workflow'),
    tag: z.string().describe('Tag to apply to the image'),
    additional_tags: z.array(z.string()).optional()
      .describe('Additional tags to apply')
  }
});