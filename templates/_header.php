<?php
function buildLinks() {
  global $selectedLink;

  $links = array(
    '' => 'Home',
    'shop.html' => 'Buy Now',
    'gallery.html' => 'Gallery',
    'about.html' => 'About',
  );

  foreach ($links as $path => $name) {
    if ($selectedLink === $path){
      echo "<a href=\"/$path\" id=\"link-selected\">$name</a>";
    } else {
      echo "<a href=\"/$path\">$name</a>";
    }
  }
}
?>

<?xml version="1.0" encoding="utf-8"?>
<!DOCTYPE html PUBLIC "-//W3C//DTD XHTML 1.0 Strict//EN"
 "http://www.w3.org/TR/xhtml1/DTD/xhtml1-strict.dtd">
<html xmlns="http://www.w3.org/1999/xhtml" lang="en" xml:lang="en">
  <head>
    <meta http-equiv="Content-Type" content="text/html;charset=utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <link rel="stylesheet" type="text/css" href="style.css" />
    <!-- Include Stripe script on all pages for fraud protection -->
    <!-- https://stripe.com/docs/web/setup -->
    <script src="https://js.stripe.com/v3/" async></script>
    <?php
    if (isset($jsHandler)) {
      echo "<script type=\"text/javascript\" src=\"$jsHandler\" async></script>";
    }
    ?>
    <title>Cosmo's Tuna</title>
  </head>
  <body>
    <header>
      <nav>
        <?php buildLinks() ?>
      </nav>
    </header>