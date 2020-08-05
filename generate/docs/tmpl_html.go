package docs

const (
	htmlTmpl = `<html>

	<head>
	    <link rel="stylesheet" href="https://stackpath.bootstrapcdn.com/bootstrap/4.1.3/css/bootstrap.min.css" integrity="sha384-MCw98/SFnGE8fJT3GXwEOngsV7Zt27NXFoaoApmYm81iuXoPkFOJwJ8ERdknLPMO" crossorigin="anonymous">
		 <link rel="stylesheet" href="https://cdnjs.cloudflare.com/ajax/libs/highlight.js/9.5.0/styles/vs.min.css">
		<style>
		.hljs{
			background:#fafafa !important;
		}
		</style>
	</head>

	<body>
	<div id="navbar-example" class="bd-example">
		<div class="row">
			<div class="col-3">
				<div style="position:fixed;" id="list-example" class="list-group col-3">
					{{range $group, $resource := .Resources}}
					<a class="list-group-item list-group-item-action" href="#{{ replace .Name "/" "-" }}">{{.Name}}</a> {{end}}
				</div>
			</div>
			<div class="col-9">
				<div data-spy="scroll" data-target="#list-example" data-offset="0" class="scrollspy-example">
					{{range $group, $resource := .Resources}}
					<div>
					<h4 id="{{ replace .Name "/" "-" }}">{{.Name}}</h4>
					<p>{{.Description}}</p>
					<pre><code class="yaml">
resources:
    - name: my-awesome-name
      type: {{ .Name }}
      options:
	  {{range .Options}}{{.Name}}: # ({{.Type}}) {{.Description}}
	  {{end}}</code></pre>
					</div>

					<h6>Actions</h6>
					<ul class="list-unstyled">
					{{range .Actions}}
					  <li style="border-bottom:solid 1px #f7f7f7;padding:10px 0px;" class="media">
					    <div style="background:#f1f1f1;padding:5px;" class="media-body">
					      <h5 class="mt-0 mb-1">{{.Name}}</h5>
					     <i>expressions</i> {{range $expr := .Expressions}} - {{ $expr }} {{end}}
					    </div>
					  </li>
					  {{end}}
					</ul>



					<hr>
                    {{end}}
				</div>
			</div>
		</div>
	</div>
	<script src="https://code.jquery.com/jquery-3.3.1.slim.min.js" integrity="sha384-q8i/X+965DzO0rT7abK41JStQIAqVgRVzpbzo5smXKp4YfRvH+8abtTE1Pi6jizo" crossorigin="anonymous"></script>
    <script src="https://stackpath.bootstrapcdn.com/bootstrap/4.1.3/js/bootstrap.min.js" integrity="sha384-ChfqqxuZUCnJSK3+MXmPNIyE6ZbWh2IMqE241rYiqJxyMiZ6OW/JmZQ5stwEULTy" crossorigin="anonymous"></script>


	<script type="text/javascript">
        $('body').scrollspy({
            target: '#navbar-example'
        })
    </script>
	<script src="https://cdnjs.cloudflare.com/ajax/libs/highlight.js/9.12.0/highlight.min.js"></script>
	<script src="https://cdnjs.cloudflare.com/ajax/libs/highlight.js/9.12.0/languages/yaml.min.js"></script>
	<script>hljs.initHighlightingOnLoad();</script>
	</body>

	</html>

`
)
