import { Component, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common'; // For *ngIf, etc.
import { ReactiveFormsModule, FormBuilder, FormGroup, Validators, FormArray } from '@angular/forms';
import { Router, RouterModule } from '@angular/router';
import { ArticleService, CreateArticleRequest, CreateArticleTranslation } from '../../services/article.service';

@Component({
  selector: 'app-article-create',
  standalone: true,
  imports: [CommonModule, ReactiveFormsModule, RouterModule],
  templateUrl: './article-create.component.html',
  styleUrls: ['./article-create.component.scss']
})
export class ArticleCreateComponent implements OnInit {
  articleForm!: FormGroup; // Definite assignment assertion
  isSubmitting = false;
  error: string | null = null;

  constructor(
    private fb: FormBuilder,
    private articleService: ArticleService,
    private router: Router
  ) { }

  ngOnInit(): void {
    this.articleForm = this.fb.group({
      slug: ['', [Validators.required, Validators.pattern(/^[a-z0-9]+(?:-[a-z0-9]+)*$/)]],
      author_id: [null as number | null, [Validators.min(1)]], // Optional author_id
      translations: this.fb.array([this.createTranslationGroup()]) // Start with one translation
    });
  }

  createTranslationGroup(): FormGroup {
    return this.fb.group({
      language_code: ['en', Validators.required], // Default to 'en'
      title: ['', Validators.required],
      content: ['', Validators.required],
      is_published: [false]
    });
  }

  get translations(): FormArray {
    return this.articleForm.get('translations') as FormArray;
  }

  addTranslation(): void {
    this.translations.push(this.createTranslationGroup());
  }

  removeTranslation(index: number): void {
    this.translations.removeAt(index);
  }

  onSubmit(): void {
    if (this.articleForm.invalid) {
      this.markFormGroupTouched(this.articleForm);
      this.error = 'Please fill in all required fields correctly.';
      return;
    }

    this.isSubmitting = true;
    this.error = null;

    const formValue = this.articleForm.value;
    const articleData: CreateArticleRequest = {
      slug: formValue.slug,
      author_id: formValue.author_id || undefined, // Send undefined if null or empty
      translations: formValue.translations.map((t: any) => ({
        language_code: t.language_code,
        title: t.title,
        content: t.content,
        is_published: t.is_published || false
      }))
    };
    
    if (!articleData.author_id) { // Ensure author_id is not sent if it's null (backend expects omitempty)
        delete articleData.author_id;
    }


    this.articleService.createArticle(articleData).subscribe({
      next: (newArticle) => {
        this.isSubmitting = false;
        this.router.navigate(['/articles', newArticle.slug]);
      },
      error: (err) => {
        this.isSubmitting = false;
        this.error = `Failed to create article: ${err.message || 'Unknown error'}`;
        if (err.error && typeof err.error === 'string') {
            this.error += ` (Server: ${err.error})`;
        } else if (err.error && err.error.message) {
            this.error += ` (Server: ${err.error.message})`;
        }
        console.error('Error creating article:', err);
      }
    });
  }

  // Helper to mark all fields in a form group as touched
  private markFormGroupTouched(formGroup: FormGroup | FormArray) {
    Object.values(formGroup.controls).forEach(control => {
      if (control instanceof FormGroup || control instanceof FormArray) {
        this.markFormGroupTouched(control);
      } else {
        control.markAsTouched();
      }
    });
  }

  // Convenience getters for template validation
  get slug() { return this.articleForm.get('slug'); }
  // Add more getters for translation fields if needed for specific error messages in template
}
