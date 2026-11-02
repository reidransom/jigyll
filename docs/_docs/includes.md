---
title: Includes
permalink: /docs/includes/
description: Reusing partials from the _includes folder with the include and include_relative tags.
---

The `include` tag allows you to include the content from another file stored
in the `_includes` folder:

{% raw %}
```liquid
{% include footer.html %}
```
{% endraw %}

Jigyll will look for the referenced file (in this case, `footer.html`) in
the `_includes` directory at the root of your source directory and insert
its contents. If your site uses a [theme](/docs/themes/), the theme's
`_includes` directory is searched as a fallback.

### Including files relative to another file

You can include file fragments relative to the current file by using the
`include_relative` tag:

{% raw %}
```liquid
{% include_relative somedir/footer.html %}
```
{% endraw %}

You won't need to place your included content within the `_includes`
directory — the inclusion is resolved relative to the file where the tag is
used. All the other capabilities of the `include` tag are available to
`include_relative`, such as variables.

### Using variable names for the include file

The name of the file you want to embed can be specified as a variable
instead of an actual file name. For example, suppose you defined a variable
in your page's front matter like this:

```yaml
---
title: My page
my_variable: footer_company_a.html
---
```

You could then reference that variable in your include:

{% raw %}
```liquid
{% if page.my_variable %}
  {% include {{ page.my_variable }} %}
{% endif %}
```
{% endraw %}

Note that the `{% raw %}{{ }}{% endraw %}` wrapper is required — a bare
`{% raw %}{% include page.my_variable %}{% endraw %}` is treated as a
literal filename.

### Passing parameters to includes

You can also pass parameters to an include. For example, suppose you have a
file called `note.html` in your `_includes` folder that contains this
formatting:

{% raw %}
```liquid
<div class="alert alert-info" role="alert">
<b>Note:</b> {{ include.content }}
</div>
```
{% endraw %}

The {% raw %}`{{ include.content }}`{% endraw %} is a parameter that gets
populated when you call the include and specify a value for it:

{% raw %}
```liquid
{% include note.html content="This is my sample note." %}
```
{% endraw %}

The value of `content` will be inserted into the
{% raw %}`{{ include.content }}`{% endraw %} parameter.

You can create includes that act as templates for a variety of uses —
inserting audio or video clips, alerts, special formatting, and more. To
safeguard situations where users don't supply a value for a parameter, use
[Liquid's default filter](https://shopify.github.io/liquid/filters/default/).

### Passing parameter variables to includes

Suppose the parameter you want to pass to the include is a variable rather
than a string — for example {% raw %}`{{ site.product_name }}`{% endraw %}.
The string you pass to your include parameter can't contain curly braces, so
store the parameter in a variable first with `capture`:

{% raw %}
```liquid
{% capture download_note %}
The latest version of {{ site.product_name }} is now available.
{% endcapture %}
```
{% endraw %}

Then pass this captured variable into the parameter, omitting the quotation
marks because it's no longer a string:

{% raw %}
```liquid
{% include note.html content=download_note %}
```
{% endraw %}
